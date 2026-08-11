import grpc
import asyncio
import logging
import json
import time
import uuid
from pathlib import Path

import agent_pb2
import agent_pb2_grpc

class FlowPartnerClient:
    def __init__(self, workspace_path: str, server_address: str = "localhost:50051"):
        self.workspace_path = Path(workspace_path)
        self.history_dir = self.workspace_path / "history"
        self.server_address = server_address
        self.channel = None
        self.stub = None
        
        # 确保 history 目录存在
        self.history_dir.mkdir(parents=True, exist_ok=True)
        logging.info(f"历史记录目录已就绪: {self.history_dir}")

    def generate_session_id(self) -> str:
        """生成唯一且较长的 Session ID"""
        timestamp = int(time.time() * 1000)
        random_str = uuid.uuid4().hex[:8]
        return f"sess_{timestamp}_{random_str}"

    async def send_event(self, queue: asyncio.Queue, session_id: str, event_type: str, payload: dict):
        """将事件放入队列，由双向流发送给 Go"""
        event = agent_pb2.AgentEvent(
            session_id=session_id,
            event_type=event_type,
            payload=json.dumps(payload, ensure_ascii=False)
        )
        await queue.put(event)
        logging.info(f"[发送事件] {event_type} | Session: {session_id}")

    async def connect_and_listen(self):
        logging.info(f"准备连接到 Go Server: {self.server_address}")
        self.channel = grpc.aio.insecure_channel(self.server_address)
        self.stub = agent_pb2_grpc.FlowPartnerServiceStub(self.channel)

        # 创建一个异步队列，用于存放 Python 想要发给 Go 的消息
        outgoing_queue = asyncio.Queue()

        async def request_generator():
            """生成器：不断从队列中取出消息发给 Go"""
            while True:
                event = await outgoing_queue.get()
                yield event

        try:
            # 建立双向流
            stream = self.stub.SyncChannel(request_generator())
            logging.info("双向流已建立，等待 Go 下发指令...")

            # 模拟：我们自己生成一个 Session ID，并主动向 Go 汇报“我准备好了”
            # (在实际业务中，可能是 Go 先发指令，这里我们先发一个心跳/状态)
            test_session = self.generate_session_id()
            await self.send_event(outgoing_queue, test_session, "status_update", {"status": "ready"})

            # 监听 Go 发来的指令
            async for command in stream:
                logging.info(f"[收到指令] Type: {command.command_type} | Session: {command.session_id}")
                
                if command.command_type == "start_chat":
                    # 开启一个异步任务来处理对话，不阻塞主接收流
                    asyncio.create_task(self.handle_chat(command, outgoing_queue))
                elif command.command_type == "cancel_task":
                    logging.info("收到取消任务指令")

        except grpc.aio.AioRpcError as e:
            logging.error(f"连接断开或发生错误: {e.code()}, {e.details()}")
        finally:
            if self.channel:
                await self.channel.close()

    async def handle_chat(self, command, queue):
        """处理一次对话的 ReAct 核心逻辑 (骨架)"""
        session_id = command.session_id
        history_file = self.history_dir / f"{session_id}.json"
        
        logging.info(f"开始处理对话 Session: {session_id}")
        
        # 1. 通知前端：开始思考
        await self.send_event(queue, session_id, "status_update", {"status": "thinking"})
        
        # 2. 模拟 ReAct 过程：决定调用工具
        await asyncio.sleep(1)
        await self.send_event(queue, session_id, "tool_call", {"tool": "read_file", "args": {"path": "test.txt"}})
        
        # 3. 模拟工具执行完毕
        await asyncio.sleep(1)
        await self.send_event(queue, session_id, "tool_result", {"result": "File content..."})
        
        # 4. 最终回答
        await self.send_event(queue, session_id, "final_answer", {"text": "这是最终回答"})
        
        # 5. 保存历史记录 (简单追加)
        with open(history_file, "a", encoding="utf-8") as f:
            f.write(json.dumps({"role": "assistant", "content": "最终回答"}) + "\n")