import grpc
import asyncio
import logging
import json
from pathlib import Path

import agent_pb2
import agent_pb2_grpc
from core.react_agent import ReactAgent
from tools.registry import ToolRegistry
from tools.file_ops import read_file, write_file, list_directory

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
            description="读取本地指定路径的文件内容",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "文件的绝对路径"}
                },
                "required": ["path"]
            },
            handler=read_file
        )
        self.tool_registry.register(
            name="write_file",
            description="将内容写入本地指定路径的文件",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "文件的绝对路径"},
                    "content": {"type": "string", "description": "要写入的内容"}
                },
                "required": ["path", "content"]
            },
            handler=write_file
        )
        self.tool_registry.register(
            name="list_directory",
            description="列出指定目录下的所有文件和子目录",
            parameters={
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "目录的绝对路径"}
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
        """通过 gRPC 请求 Go 代为调用大模型"""
        try:
            request = agent_pb2.LLMRequest(
                session_id=session_id,
                json_payload=json_payload
            )
            response = await self.stub.CallLLM(request)
            return {
                "success": response.success,
                "error_message": response.error_message,
                "json_response": response.json_response
            }
        except Exception as e:
            return {"success": False, "error_message": str(e), "json_response": ""}

    async def connect_and_listen(self):
        logging.info(f"准备连接到 Go Server: {self.server_address}")
        self.channel = grpc.aio.insecure_channel(self.server_address)
        self.stub = agent_pb2_grpc.FlowPartnerServiceStub(self.channel)

        outgoing_queue = asyncio.Queue()

        async def request_generator():
            while True:
                event = await outgoing_queue.get()
                yield event

        try:
            stream = self.stub.SyncChannel(request_generator())
            logging.info("双向流已建立，等待 Go 下发指令...")

            async for command in stream:
                logging.info(f"[收到指令] {command.command_type} | Session: {command.session_id}")

                if command.command_type == "start_chat":
                    asyncio.create_task(
                        self.handle_chat(command, outgoing_queue)
                    )

        except grpc.aio.AioRpcError as e:
            logging.error(f"连接断开: {e.code()}, {e.details()}")
        finally:
            if self.channel:
                await self.channel.close()

    async def handle_chat(self, command, queue):
        """处理一次完整的对话"""
        session_id = command.session_id
        payload = json.loads(command.payload) if command.payload else {}
        user_message = payload.get("user_message", "")

        logging.info(f"开始对话 | Session: {session_id} | 用户: {user_message}")

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

        logging.info(f"对话完成 | Session: {session_id}")