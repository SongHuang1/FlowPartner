import grpc
import asyncio
import logging

import agent_pb2
import agent_pb2_grpc

class FlowPartnerClient:
    def __init__(self, workspace_path: str, server_address: str = "localhost:50051"):
        self.workspace_path = workspace_path
        self.server_address = server_address
        self.channel = None
        self.stub = None

    async def connect_and_listen(self):
        """连接到 Go 服务器并监听任务"""
        logging.info(f"准备连接到 Go Server: {self.server_address}")
        
        self.channel = grpc.aio.insecure_channel(self.server_address)
        self.stub = agent_pb2_grpc.FlowPartnerServiceStub(self.channel)

        request = agent_pb2.RegisterRequest(
            agent_version="0.1.0", # TODO: 这个不能硬编码
            workspace_path=self.workspace_path
        )

        try:
            # 调用流式接口，Go 会不断推送 TaskCommand 给这个 task_stream
            task_stream = self.stub.ReceiveTasks(request)
            logging.info("已成功连接，正在等待 Go 下发任务...")

            async for task in task_stream:
                logging.info(f"收到新任务! ID: {task.task_id}, 类型: {task.task_type}")
                # TODO: 未来在这里根据 task_type 分发任务给不同的处理模块
                await self.process_task(task)

        except grpc.aio.AioRpcError as e:
            logging.error(f"连接断开或发生错误: {e.code()}, {e.details()}")
            # TODO: 未来这里可以加入断线重连机制
        finally:
            if self.channel:
                await self.channel.close()

    async def process_task(self, task):
        """处理单个任务的模拟逻辑"""
        logging.info(f"开始执行任务: {task.task_id} ...")
        await asyncio.sleep(2) # 模拟耗时操作
        
        # 执行完毕后，向 Go 汇报结果
        await self.submit_result(task.task_id, "任务执行成功")

    async def submit_result(self, task_id: str, result_msg: str):
        """提交结果给 Go"""
        result = agent_pb2.TaskResult(
            task_id=task_id,
            success=True,
            message=result_msg,
            result_data="{}"
        )
        try:
            response = await self.stub.SubmitResult(result)
            logging.info(f"结果提交成功: {response.received}")
        except Exception as e:
            logging.error(f"提交结果失败: {e}")