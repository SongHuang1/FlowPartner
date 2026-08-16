import logging
from collections.abc import Callable


class ToolRegistry:
    """工具注册表：管理所有 Agent 可用的本地工具"""

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
