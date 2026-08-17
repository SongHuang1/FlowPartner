import logging
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from agent.grpc_client import FlowPartnerClient


def make_read_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 read 工具 handler。"""

    async def read(path: str) -> str:
        logging.info(f"[Tool] Read (via Go): {path}")
        result = await client.execute_tool("", "read", {"path": path})
        return result["result"]

    return read


def make_write_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 write 工具 handler。"""

    async def write(path: str, content: str) -> str:
        logging.info(f"[Tool] Write (via Go): {path}")
        result = await client.execute_tool("", "write", {"path": path, "content": content})
        return result["result"]

    return write


def make_bash_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 bash 工具 handler。"""

    async def bash(command: str) -> str:
        logging.info(f"[Tool] Bash (via Go): {command}")
        result = await client.execute_tool("", "bash", {"command": command})
        return result["result"]

    return bash


def make_edit_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 edit 工具 handler。"""

    async def edit(path: str, old_string: str, new_string: str) -> str:
        logging.info(f"[Tool] Edit (via Go): {path}")
        result = await client.execute_tool("", "edit", {
            "path": path,
            "old_string": old_string,
            "new_string": new_string,
        })
        return result["result"]

    return edit
