import asyncio
import json
import logging
import re
from pathlib import Path

import grpc

from agent import agent_pb2, agent_pb2_grpc
from agent.core.react_agent import ReactAgent
from agent.tools.file_ops import list_directory, read_file, write_file
from agent.tools.registry import ToolRegistry

# 会话 ID 白名单：仅允许字母数字、下划线、连字符（与 Go 侧 storage.ValidSessionID 一致），
# 防止被用作文件名时发生路径遍历
_SESSION_ID_RE = re.compile(r"^[a-zA-Z0-9_-]{1,128}$")


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
        # 串行化历史文件写入，避免同一会话并发追加时交错
        self._history_lock = asyncio.Lock()

        # 初始化并注册所有工具
        self.tool_registry = ToolRegistry()
        self.tool_registry.register(
            name="read_file",
            description="Read the content of a local file at the specified path",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "Absolute path to the file"}
                },
                "required": ["path"]
            },
            handler=read_file
        )
        self.tool_registry.register(
            name="write_file",
            description="Write content to a local file at the specified path",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "Absolute path to the file"},
                    "content": {"type": "string", "description": "Content to write"}
                },
                "required": ["path", "content"]
            },
            handler=write_file
        )
        self.tool_registry.register(
            name="list_directory",
            description="List all files and subdirectories in the specified directory",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "Absolute path to the directory"}
                },
                "required": ["path"]
            },
            handler=list_directory
        )

    async def send_event(self, session_id: str, event_type: str, payload: dict, queue: asyncio.Queue):
        event = agent_pb2.AgentEvent(
            session_id=session_id,
            event_type=event_type,
            payload=json.dumps(payload, ensure_ascii=False)
        )
        await queue.put(event)

    async def call_llm_via_go(self, session_id: str, json_payload: str, send_event_func=None) -> dict:
        """通过 gRPC 请求 Go 代为调用大模型（服务端流式）"""
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
                            "type": tc.get("type", "function"),
                            "function": {
                                "name": tc.get("function", {}).get("name", ""),
                                "arguments": ""
                            }
                        }
                    func = tc.get("function")
                    if func and func.get("arguments"):
                        tool_calls_map[idx]["function"]["arguments"] += func["arguments"]

                if choices[0].get("finish_reason"):
                    finish_reason = choices[0]["finish_reason"]

                if chunk.get("usage"):
                    usage = chunk["usage"]

                if send_event_func:
                    await send_event_func(session_id, "llm_chunk", {
                        "content": delta.get("content", ""),
                        "finish_reason": choices[0].get("finish_reason")
                    })

            result: dict = {
                "success": True,
                "content": full_content,
                "finish_reason": finish_reason,
                "json_response": ""
            }
            if tool_calls_map:
                result["tool_calls"] = [tool_calls_map[i] for i in sorted(tool_calls_map)]
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

            except grpc.aio.AioRpcError as e:
                logging.warning(f"Connection lost (attempt {attempt}/{max_retries}): {e.code()}, {e.details()}")
            except Exception as e:
                logging.error(f"Unexpected error (attempt {attempt}/{max_retries}): {e}")
            finally:
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
        """处理一次完整的对话"""
        session_id = command.session_id
        try:
            session_id = _safe_session_id(session_id)
        except ValueError:
            logging.error(f"Rejecting chat with invalid session id: {session_id!r}")
            return

        payload = json.loads(command.payload) if command.payload else {}
        user_message = payload.get("user_message", "")
        history = payload.get("history", [])
        if not isinstance(history, list):
            history = []

        logging.info(f"[Chat started] Session: {session_id} | User: {user_message}")

        # 创建 ReAct Agent 实例
        agent = ReactAgent(
            session_id=session_id,
            call_llm_func=self.call_llm_via_go,
            send_event_func=lambda sid, etype, epayload: self.send_event(sid, etype, epayload, queue),
            tool_registry=self.tool_registry
        )
        # 执行 ReAct 循环，携带同一会话的历史上下文
        final_answer = await agent.run(
            user_message,
            history=history,
            send_event_func=lambda sid, etype, epayload: self.send_event(sid, etype, epayload, queue),
        )

        # 保存历史记录（JSONL 追加：每行一个 [user, assistant] 数组）
        history_file = self.history_dir / f"{session_id}.json"
        async with self._history_lock:
            with open(history_file, "a", encoding="utf-8") as f:
                f.write(json.dumps([
                    {"role": "user", "content": user_message},
                    {"role": "assistant", "content": final_answer}
                ], ensure_ascii=False) + "\n")

        logging.info(f"Chat completed | Session: {session_id}")
