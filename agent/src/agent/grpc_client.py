import grpc
import asyncio
import logging
import json

import agent_pb2
import agent_pb2_grpc

# --- 任务处理模块 (未来这里会拆分成独立的文件) ---

async def handle_test_echo(payload: dict) -> str:
    """处理测试回显任务"""
    msg = payload.get("message", "empty")
    logging.info(f"[执行 test_echo] 收到消息: {msg}")
    await asyncio.sleep(1) # 模拟耗时
    return f"Echo: {msg} (Processed by Python)"

async def handle_read_file(payload: dict) -> str:
    """(预留) 处理读取本地文件任务"""
    file_path = payload.get("path")
    logging.info(f"[执行 read_file] 准备读取: {file_path}")
    # TODO: 未来在这里实现真实的文件读取
    return "File content mock"

# --- 任务分发器 ---

class TaskDispatcher:
    def __init__(self):
        # 注册 task_type 到具体的处理函数
        self._handlers = {
            "test_echo": handle_test_echo,
            "read_file": handle_read_file,
            # 未来新增任务只需在这里加一行
        }

    async def dispatch(self, task_type: str, payload_str: str) -> tuple[bool, str, str]:
        """
        分发任务并返回结果
        :return: (success: bool, message: str, result_data: str)
        """
        handler = self._handlers.get(task_type)
        if not handler:
            return False, f"未知的任务类型: {task_type}", ""

        try:
            payload = json.loads(payload_str) if payload_str else {}
            # 执行具体的处理函数
            result_data = await handler(payload)
            return True, "执行成功", result_data
        except Exception as e:
            logging.error(f"任务执行异常: {e}", exc_info=True)
            return False, f"执行异常: {str(e)}", ""

# --- gRPC 客户端 ---

class FlowPartnerClient:
    def __init__(self, workspace_path: str, server_address: str = "localhost:50051"):
        self.workspace_path = workspace_path
        self.server_address = server_address
        self.channel = None
        self.stub = None
        self.dispatcher = TaskDispatcher() # 初始化分发器

    async def connect_and_listen(self):
        logging.info(f"准备连接到 Go Server: {self.server_address}")
        self.channel = grpc.aio.insecure_channel(self.server_address)
        self.stub = agent_pb2_grpc.FlowPartnerServiceStub(self.channel)

        request = agent_pb2.RegisterRequest(
            agent_version="0.1.0",
            workspace_path=self.workspace_path
        )

        try:
            task_stream = self.stub.ReceiveTasks(request)
            logging.info("已成功连接，正在等待 Go 下发任务...")

            async for task in task_stream:
                logging.info(f"收到新任务! ID: {task.task_id}, 类型: {task.task_type}")
                
                # 使用分发器处理任务
                success, message, result_data = await self.dispatcher.dispatch(
                    task.task_type, task.payload
                )
                
                # 汇报结果
                await self.submit_result(task.task_id, success, message, result_data)

        except grpc.aio.AioRpcError as e:
            logging.error(f"连接断开或发生错误: {e.code()}, {e.details()}")
        finally:
            if self.channel:
                await self.channel.close()

    async def submit_result(self, task_id: str, success: bool, message: str, result_data: str):
        result = agent_pb2.TaskResult(
            task_id=task_id,
            success=success,
            message=message,
            result_data=result_data
        )
        try:
            response = await self.stub.SubmitResult(result)
            logging.info(f"结果提交成功: {response.received}")
        except Exception as e:
            logging.error(f"提交结果失败: {e}")