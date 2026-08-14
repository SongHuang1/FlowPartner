import asyncio
import json
import logging
from pathlib import Path

import grpc
import agent_pb2
import agent_pb2_grpc
from core.react_agent import ReactAgent
from tools.file_ops import list_directory, read_file, write_file
from tools.registry import ToolRegistry

class FlowPartnerClient:
    def __init__(self, workspace_path: str, server_address: str = "localhost:50051"):
        self.workspace_path = Path(workspace_path)
        self.history_dir = self.workspace_path / "history"
        self.server_address = server_address
        self.channel = None
        self.stub = None
        self.history_dir.mkdir(parents=True, exist_ok=True)

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

    async def call_llm_via_go(self, session_id: str, json_payload: str) -> dict:
        """通过 gRPC 请求 Go 代为调用大模型（服务端流式）"""
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
                        "error_message": error_data.get("message", "Unknown error"),
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

                await self.send_event(session_id, "llm_chunk", {
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
        self.channel = grpc.aio.insecure_channel(self.server_address)
        self.stub = agent_pb2_grpc.FlowPartnerServiceStub(self.channel)

        outgoing_queue = asyncio.Queue()

        async def request_generator():
            while True:
                event = await outgoing_queue.get()
                yield event

        try:
            stream = self.stub.SyncChannel(request_generator())
            logging.info("Bidirectional stream established, waiting for Go commands...")

            async for command in stream:
                logging.info(f"[Command received] {command.command_type} | Session: {command.session_id}")

                if command.command_type == "start_chat":
                    asyncio.create_task(
                        self.handle_chat(command, outgoing_queue)
                    )

        except grpc.aio.AioRpcError as e:
            logging.error(f"Connection lost: {e.code()}, {e.details()}")
        finally:
            if self.channel:
                await self.channel.close()

    async def handle_chat(self, command, queue):
        """处理一次完整的对话"""
        session_id = command.session_id
        payload = json.loads(command.payload) if command.payload else {}
        user_message = payload.get("user_message", "")

        logging.info(f"Chat started | Session: {session_id} | User: {user_message}")

        # 创建 ReAct Agent 实例
        agent = ReactAgent(
            session_id=session_id,
            call_llm_func=self.call_llm_via_go,
            send_event_func=lambda sid, etype, epayload: self.send_event(sid, etype, epayload, queue),
            tool_registry=self.tool_registry
        )

        # 执行 ReAct 循环
        final_answer = await agent.run(user_message)

        # 保存历史记录
        history_file = self.history_dir / f"{session_id}.json"
        with open(history_file, "a", encoding="utf-8") as f:
            f.write(json.dumps([
                {"role": "user", "content": user_message},
                {"role": "assistant", "content": final_answer}
            ], ensure_ascii=False) + "\n")

        logging.info(f"Chat completed | Session: {session_id}")