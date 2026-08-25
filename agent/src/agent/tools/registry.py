import logging
from collections.abc import Callable


class ToolRegistry:
    """工具注册表：管理暴露给 LLM 的全部可调用项（read/write/bash/edit/trash/purge 经 gRPC 由 Go 代理执行，agent__* 为子智能体调度入口）。"""

    def __init__(self):
        self._tools: dict[str, dict] = {}

    def register(self, name: str, description: str, parameters: dict, handler: Callable):
        """注册一个工具"""
        self._tools[name] = {
            "description": description,
            "parameters": parameters,
            "handler": handler
        }
        logging.info(f"Tool registered: {name}")

    def copy(self) -> "ToolRegistry":
        """浅拷贝工具注册表（handler 共享，供子 agent 各自重建工具集）。"""
        cloned = ToolRegistry()
        cloned._tools = dict(self._tools)
        return cloned

    def get_openai_tools_definition(self) -> list[dict]:
        """生成 OpenAI 格式的 tools 定义"""
        return [
            {
                "type": "function",
                "function": {
                    "name": name,
                    "description": info["description"],
                    "parameters": info["parameters"]
                }
            }
            for name, info in self._tools.items()
        ]

    async def execute(self, name: str, arguments: dict) -> str:
        """执行指定工具"""
        tool = self._tools.get(name)
        if not tool:
            return f"错误：未知工具 '{name}'"
        try:
            result = await tool["handler"](**arguments)
            return str(result)
        except Exception as e:
            logging.error(f"Tool {name} execution error: {e}", exc_info=True)
            return f"工具执行错误：{str(e)}"
