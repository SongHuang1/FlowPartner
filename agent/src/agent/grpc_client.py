import asyncio
import json
import logging
import re
import uuid
from pathlib import Path

import grpc

from agent import agent_pb2, agent_pb2_grpc
from agent.core.agent_registry import AgentRegistry
from agent.core.react_agent import ReactAgent
from agent.tools.file_ops import (
    make_bash_handler,
    make_edit_handler,
    make_purge_handler,
    make_read_handler,
    make_trash_handler,
    make_write_handler,
)
from agent.tools.registry import ToolRegistry

_SESSION_ID_RE = re.compile(r"^[a-zA-Z0-9_-]{1,128}$")

_CANCELLED_ANSWER = "已停止生成"


def _safe_session_id(session_id: str) -> str:
    if not session_id or not _SESSION_ID_RE.fullmatch(session_id):
        raise ValueError(f"Invalid session id: {session_id!r}")
    return session_id


class FlowPartnerClient:
    def __init__(self, workspace_path: str, server_address: str = "localhost:50051"):
        self.workspace_path = Path(workspace_path)
        self.history_dir = self.workspace_path / "history"
        self.server_address = server_address
        self.channel: grpc.aio.Channel | None = None
        self.stub: agent_pb2_grpc.FlowPartnerServiceStub | None = None
        self.history_dir.mkdir(parents=True, exist_ok=True)
        self._history_lock = asyncio.Lock()

        self._pending_approvals: dict[str, asyncio.Future] = {}
        self._approval_lock = asyncio.Lock()

        self._tasks: dict[str, asyncio.Task] = {}
        self._active_queues: dict[str, asyncio.Queue] = {}

        self.tool_registry = ToolRegistry()
        self.agent_registry = AgentRegistry(self)
        self.tool_registry.register(
            name="read",
            description="Read the content of a local file at the specified path",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "File path (absolute or relative to working directory)"}
                },
                "required": ["path"]
            },
            handler=make_read_handler(self)
        )
        self.tool_registry.register(
            name="write",
            description="Write content to a local file at the specified path. Creates parent directories automatically.",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "File path (absolute or relative to working directory)"},
                    "content": {"type": "string", "description": "Content to write"}
                },
                "required": ["path", "content"]
            },
            handler=make_write_handler(self)
        )
        self.tool_registry.register(
            name="bash",
            description="Execute a shell command in the working directory. Timeout: 30 seconds.",
            parameters={
                "type": "object",
                "properties": {
                    "command": {"type": "string", "description": "Shell command to execute"}
                },
                "required": ["command"]
            },
            handler=make_bash_handler(self)
        )
        self.tool_registry.register(
            name="edit",
            description="Search for old_string in a file and replace with new_string. Must match exactly once.",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "File path (absolute or relative to working directory)"},
                    "old_string": {"type": "string", "description": "String to search for (must match exactly once)"},
                    "new_string": {"type": "string", "description": "Replacement string"}
                },
                "required": ["path", "old_string", "new_string"]
            },
            handler=make_edit_handler(self)
        )
        self.tool_registry.register(
            name="trash",
            description=(
                "Move files or directories to the recycle bin (trash) instead of permanently deleting them. "
                "Use this tool whenever you need to delete files. Pass 'path' for a single item, or 'paths' "
                "for multiple items. Deleted items can be recovered later."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "Single file/dir path to move to trash (absolute or relative to working directory)"},
                    "paths": {"type": "array", "items": {"type": "string"}, "description": "Multiple paths to move to trash"}
                },
                "required": []
            },
            handler=make_trash_handler(self)
        )
        self.tool_registry.register(
            name="purge",
            description=(
                "Permanently delete items from the recycle bin (trash). Requires explicit user approval every time. "
                "Use ONLY when the user explicitly asks to permanently delete trashed files. "
                "Pass 'entry' to delete a single trashed item by name, or no arguments to empty the entire trash."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "entry": {"type": "string", "description": "Name of a single trashed item to permanently delete; omit to empty the entire trash"}
                },
                "required": []
            },
            handler=make_purge_handler(self)
        )

    async def list_agents(self) -> list[dict]:

        if self.stub is None:
            raise grpc.aio.AioRpcError(
                grpc.StatusCode.UNAVAILABLE, None, None, "gRPC 连接未建立"
            )
        response = await self.stub.ListAgents(agent_pb2.Empty())
        return [
            {
                "id": agent.id,
                "name": agent.name,
                "description": agent.description,
                "system_prompt": agent.system_prompt,
                "created_at": agent.created_at,
                "updated_at": agent.updated_at,
            }
            for agent in response.agents
        ]

    async def get_agent(self, agent_id: str) -> dict | None:
        if self.stub is None:
            raise grpc.aio.AioRpcError(
                grpc.StatusCode.UNAVAILABLE, None, None, "gRPC 连接未建立"
            )
        try:
            response = await self.stub.GetAgent(agent_pb2.AgentId(id=agent_id))
        except grpc.aio.AioRpcError as e:
            if e.code() == grpc.StatusCode.NOT_FOUND:
                return None
            raise
        return {
            "id": response.id,
            "name": response.name,
            "description": response.description,
            "system_prompt": response.system_prompt,
            "created_at": response.created_at,
            "updated_at": response.updated_at,
        }

    async def send_event(self, session_id: str, event_type: str, payload: dict, queue: asyncio.Queue):
        event = agent_pb2.AgentEvent(
            session_id=session_id,
            event_type=event_type,
            payload=json.dumps(payload, ensure_ascii=False)
        )
        await queue.put(event)

    async def execute_tool(self, session_id: str, tool_name: str, arguments: dict) -> dict:
        if self.stub is None:
            return {"success": False, "result": "gRPC 连接未建立，请稍后重试", "error_code": "TOOL_ERROR"}
        try:
            request = agent_pb2.ToolRequest(
                session_id=session_id,
                tool_name=tool_name,
                arguments=json.dumps(arguments, ensure_ascii=False)
            )
            response = await self.stub.ExecuteTool(request)

            if response.needs_permission:
                request_id = response.request_id
                logging.info(f"[Permission] Tool {tool_name} needs approval: request_id={request_id}")

                path = arguments.get("path", "")
                operation_map = {"read": "读取", "write": "写入", "edit": "编辑"}
                operation = operation_map.get(tool_name, tool_name)

                event_payload = {
                    "request_id": request_id,
                    "tool": tool_name,
                    "path": path,
                    "operation": operation,
                    "detail": f"Agent 想要{operation}路径 {path}",
                    "scope_options": ["once", "session"],
                }

                await self._send_permission_request(session_id, event_payload)

                future: asyncio.Future = asyncio.get_running_loop().create_future()
                async with self._approval_lock:
                    self._pending_approvals[request_id] = future

                decision = await future

                if decision == "allow":
                    retry_request = agent_pb2.ToolRequest(
                        session_id=session_id,
                        tool_name=tool_name,
                        arguments=json.dumps(arguments, ensure_ascii=False),
                        approval_id=request_id,
                    )
                    retry_response = await self.stub.ExecuteTool(retry_request)
                    return {
                        "success": retry_response.success,
                        "result": retry_response.result,
                        "error_code": retry_response.error_code,
                    }
                else:
                    path = arguments.get("path", "")
                    return {"success": False, "result": f"用户拒绝了此操作：{path}", "error_code": "PERMISSION_DENIED"}

            return {
                "success": response.success,
                "result": response.result,
                "error_code": response.error_code,
            }
        except grpc.aio.AioRpcError as e:
            return {"success": False, "result": f"gRPC 通信失败: {e.details()}", "error_code": "TOOL_ERROR"}
        except Exception as e:
            return {"success": False, "result": f"工具调用异常: {e}", "error_code": "TOOL_ERROR"}

    async def _send_permission_request(self, session_id: str, payload: dict) -> None:
        queue = self._active_queues.get(session_id)
        if queue is None:
            logging.warning(f"[Permission] No active queue for session {session_id}, cannot send permission_request")
            return
        await self.send_event(session_id, "permission_request", payload, queue)

    def _drain_pending_approvals(self) -> None:
        for request_id, future in list(self._pending_approvals.items()):
            if not future.done():
                future.set_result("deny")
                logging.info(f"[Permission] Drained pending approval on disconnect: request_id={request_id}")
        self._pending_approvals.clear()

    async def call_llm_via_go(self, session_id: str, json_payload: str, send_event_func=None, iteration: int = 0) -> dict:
        """通过 gRPC 请求 Go 代为调用大模型（服务端流式）。

        成功时返回: success=True, content(完整文本), finish_reason,
        json_response(恒为空串), 以及可选的 tool_calls(已剔除参数非法/无名目的条目)、usage。
        失败时返回: success=False, error_message, error_guess(原因猜测，可能为空), json_response=""。
        """
        if self.stub is None:
            return {"success": False, "error_message": "gRPC 连接未建立，请稍后重试", "error_guess": "", "json_response": ""}
        try:
            request = agent_pb2.LLMRequest(
                session_id=session_id,
                json_payload=json_payload
            )
            full_content = ""
            tool_calls_map: dict[int, dict] = {}
            finish_reason = ""
            usage: dict | None = None

            async for response in self.stub.CallLLM(request):
                if response.is_error:
                    error_data = json.loads(response.json_response)
                    return {
                        "success": False,
                        "error_message": error_data.get("message", "未知错误"),
                        "error_guess": error_data.get("guess", ""),
                        "json_response": ""
                    }

                chunk = json.loads(response.json_response)
                choices = chunk.get("choices", [])
                if not choices:
                    continue

                delta = choices[0].get("delta")
                if not delta:
                    continue

                if delta.get("content"):
                    full_content += delta["content"]

                for tc in delta.get("tool_calls", []):
                    idx = tc.get("index", 0)
                    if idx not in tool_calls_map:
                        tool_calls_map[idx] = {
                            "id": tc.get("id", ""),
                            "type": tc.get("type", ""),
                            "function": {
                                "name": "",
                                "arguments": ""
                            }
                        }
                    entry = tool_calls_map[idx]
                    if tc.get("id"):
                        entry["id"] = tc["id"]
                    if tc.get("type"):
                        entry["type"] = tc["type"]
                    func = tc.get("function")
                    if func:
                        if func.get("name"):
                            entry["function"]["name"] = func["name"]
                        if func.get("arguments"):
                            entry["function"]["arguments"] += func["arguments"]

                if choices[0].get("finish_reason"):
                    finish_reason = choices[0]["finish_reason"]

                if chunk.get("usage"):
                    usage = chunk["usage"]

                if send_event_func and delta.get("content"):
                    await send_event_func(session_id, "llm_chunk", {
                        "content": delta["content"],
                        "iteration": iteration,
                    })

            if tool_calls_map:
                valid_calls = []
                for i in sorted(tool_calls_map):
                    tc = tool_calls_map[i]
                    args_str = tc["function"]["arguments"]
                    if args_str:
                        try:
                            json.loads(args_str)
                        except json.JSONDecodeError:
                            logging.warning(
                                f"[LLM] Tool call {tc['function']['name']} has invalid JSON arguments "
                                f"({args_str[:100]}...), dropping this tool call"
                            )
                            continue
                    if not tc["function"]["name"]:
                        logging.warning(f"[LLM] Tool call at index {i} has no function name, dropping")
                        continue
                    valid_calls.append(tc)
                tool_calls_list = valid_calls
            else:
                tool_calls_list = []

            result: dict = {
                "success": True,
                "content": full_content,
                "finish_reason": finish_reason,
                "json_response": ""
            }
            if tool_calls_list:
                result["tool_calls"] = tool_calls_list
            if usage:
                result["usage"] = usage
            return result

        except grpc.aio.AioRpcError as e:
            return {"success": False, "error_message": f"gRPC error: {e.details()}", "error_guess": "", "json_response": ""}
        except (json.JSONDecodeError, KeyError) as e:
            return {"success": False, "error_message": f"Response parse error: {e}", "error_guess": "", "json_response": ""}

    async def connect_and_listen(self):
        logging.info(f"Preparing to connect to Go server: {self.server_address}")
        max_retries = 10
        retry_delay = 2

        for attempt in range(1, max_retries + 1):
            try:
                self.channel = grpc.aio.insecure_channel(self.server_address)
                self.stub = agent_pb2_grpc.FlowPartnerServiceStub(self.channel)

                outgoing_queue = asyncio.Queue()

                async def request_generator():
                    while True:
                        event = await outgoing_queue.get()
                        yield event

                stream = self.stub.SyncChannel(request_generator())
                logging.info("Bidirectional stream established, waiting for Go commands...")

                async for command in stream:
                    logging.info(f"[Command received] {command.command_type} | Session: {command.session_id}")

                    if command.command_type == "start_chat":
                        asyncio.create_task(
                            self.handle_chat(command, outgoing_queue)
                        )

                    elif command.command_type == "permission_response":
                        try:
                            payload = json.loads(command.payload) if command.payload else {}
                            request_id = payload.get("request_id", "")
                            decision = payload.get("decision", "deny")

                            async with self._approval_lock:
                                future = self._pending_approvals.pop(request_id, None)

                            if future is not None and not future.done():
                                future.set_result(decision)
                                logging.info(f"[Permission] Resolved: request_id={request_id} decision={decision}")
                            elif future is None:
                                logging.warning(f"[Permission] Unknown request_id: {request_id}")
                        except Exception as e:
                            logging.error(f"[Permission] Failed to process response: {e}")

                    elif command.command_type == "cancel_task":
                        session_id = command.session_id
                        task = self._tasks.get(session_id)
                        if task is not None and not task.done():
                            task.cancel()
                            logging.info(f"[Cancel] Task cancelled: session={session_id}")
                        else:
                            logging.info(f"[Cancel] No active task for session={session_id}, ignoring")

                    elif command.command_type == "agents_changed":
                        self.agent_registry.invalidate()

            except grpc.aio.AioRpcError as e:
                logging.warning(f"Connection lost (attempt {attempt}/{max_retries}): {e.code()}, {e.details()}")
            except Exception as e:
                logging.error(f"Unexpected error (attempt {attempt}/{max_retries}): {e}")
            finally:
                self._drain_pending_approvals()
                self._active_queues.clear()

                if self.channel:
                    await self.channel.close()
                    self.channel = None
                    self.stub = None

            if attempt < max_retries:
                logging.info(f"Retrying in {retry_delay} seconds...")
                await asyncio.sleep(retry_delay)
            else:
                logging.error("Max retries reached, giving up.")

    async def handle_chat(self, command, queue):
        session_id = command.session_id
        try:
            session_id = _safe_session_id(session_id)
        except ValueError:
            logging.error(f"Rejecting chat with invalid session id: {session_id!r}")
            return

        self._active_queues[session_id] = queue
        task = asyncio.current_task()
        self._tasks[session_id] = task

        try:
            payload = json.loads(command.payload) if command.payload else {}
            user_message = payload.get("user_message", "")
            history = payload.get("history", [])
            if not isinstance(history, list):
                history = []


            executor_agent_id = payload.get("executor_agent_id") or "main"
            inject_agent_id = payload.get("inject_agent_id") or ""

            logging.info(f"[Chat started] Session: {session_id} | User: {user_message[:200]} | Executor: {executor_agent_id} | Inject: {inject_agent_id}")

            trace_id = str(uuid.uuid4())

            async def send_evt(sid: str, etype: str, epayload: dict) -> None:
                await self.send_event(sid, etype, epayload, queue)

            system_prompt = await self.agent_registry.system_prompt_for(executor_agent_id)
            tool_registry = await self.agent_registry.build_tool_registry(
                executor_agent_id, session_id, 1, trace_id, "", send_evt
            )

            forced_tool_call = None
            if inject_agent_id:
                if inject_agent_id == executor_agent_id:
                    logging.warning(f"[Chat] Rejected: cannot inject into executor itself ({executor_agent_id})")
                    await self.send_event(
                        session_id, "error",
                        {"message": f"不能强制调用当前会话执行者自身（{executor_agent_id}），请选择其他智能体。"},
                        queue,
                    )
                    return
                target_def = await self.agent_registry.get(inject_agent_id)
                if target_def is None:
                    logging.warning(f"[Chat] Rejected: inject target not found: {inject_agent_id}")
                    await self.send_event(
                        session_id, "error",
                        {"message": f"智能体不存在或已被删除，无法调用：{inject_agent_id}"},
                        queue,
                    )
                    return
                forced_tool_call = {"name": f"agent__{inject_agent_id}", "arguments": {"task": user_message}}

            # 跟踪子智能体调用结果
            subagent_results = {}  # span_id -> {agent_name, task, content, status}

            async def tracking_send_evt(sid: str, etype: str, epayload: dict) -> None:
                # 拦截子智能体事件，记录结果
                if etype == "subagent_start":
                    span_id = epayload.get("span_id", "")
                    if span_id:
                        subagent_results[span_id] = {
                            "span_id": span_id,
                            "agent_name": epayload.get("agent_name", ""),
                            "task": epayload.get("task", ""),
                            "content": "",
                            "status": "running",
                        }
                elif etype == "subagent_step":
                    span_id = epayload.get("span_id", "")
                    if span_id and span_id in subagent_results:
                        if epayload.get("step_type") == "final_answer":
                            subagent_results[span_id]["content"] = epayload.get("content", "")
                elif etype == "subagent_end":
                    span_id = epayload.get("span_id", "")
                    if span_id and span_id in subagent_results:
                        subagent_results[span_id]["status"] = "done"
                        if not subagent_results[span_id]["content"]:
                            subagent_results[span_id]["content"] = epayload.get("result", "")
                elif etype == "subagent_error":
                    span_id = epayload.get("span_id", "")
                    if span_id and span_id in subagent_results:
                        subagent_results[span_id]["status"] = "error"
                        subagent_results[span_id]["content"] = epayload.get("message", "")
                # 转发原始事件
                await send_evt(sid, etype, epayload)

            agent = ReactAgent(
                session_id=session_id,
                call_llm_func=self.call_llm_via_go,
                send_event_func=tracking_send_evt,
                tool_registry=tool_registry
            )
            final_answer, all_messages = await agent.run(
                user_message,
                history=history,
                send_event_func=tracking_send_evt,
                system_prompt=system_prompt,
                forced_tool_call=forced_tool_call,
            )

            # 将子智能体结果附加到最后一条 assistant 消息
            results_list = list(subagent_results.values())
            if results_list and all_messages:
                # 找到最后一条 assistant 消息
                for i in range(len(all_messages) - 1, -1, -1):
                    if all_messages[i].get("role") == "assistant":
                        all_messages[i]["subagent_results"] = results_list
                        break

            history_file = self.history_dir / f"{session_id}.json"
            async with self._history_lock:
                with open(history_file, "a", encoding="utf-8") as f:
                    for msg in all_messages:
                        f.write(json.dumps(msg, ensure_ascii=False) + "\n")

                    f.write(json.dumps({"meta": {"executor_agent_id": executor_agent_id}}, ensure_ascii=False) + "\n")

            logging.info(f"Chat completed | Session: {session_id}")

        except asyncio.CancelledError:
            logging.info(f"[Cancel] Chat task cancelled: session={session_id}")
            raise
        finally:
            self._tasks.pop(session_id, None)
            self._active_queues.pop(session_id, None)
