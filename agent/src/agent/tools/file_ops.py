import json
import logging
from typing import TYPE_CHECKING

from agent.tools.context import current_session_id

if TYPE_CHECKING:
    from agent.grpc_client import FlowPartnerClient


def _format_result(result: dict) -> str:
    """将 Go 端工具结果格式化为对 LLM 友好的 JSON（含错误码，供模型决策）。"""
    return json.dumps({
        "success": result.get("success", False),
        "result": result.get("result", ""),
        "error_code": result.get("error_code", ""),
    }, ensure_ascii=False)


def make_read_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 read 工具 handler。"""

    async def read(path: str) -> str:
        logging.info(f"[Tool] Read (via Go): {path}")
        result = await client.execute_tool(current_session_id(), "read", {"path": path})
        return result["result"]

    return read


def make_write_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 write 工具 handler。"""

    async def write(path: str, content: str) -> str:
        logging.info(f"[Tool] Write (via Go): {path}")
        result = await client.execute_tool(current_session_id(), "write", {"path": path, "content": content})
        return result["result"]

    return write


def make_bash_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 bash 工具 handler。"""

    async def bash(command: str) -> str:
        logging.info(f"[Tool] Bash (via Go): {command}")
        result = await client.execute_tool(current_session_id(), "bash", {"command": command})
        return _format_result(result)

    return bash


def make_edit_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 edit 工具 handler。"""

    async def edit(path: str, old_string: str, new_string: str) -> str:
        logging.info(f"[Tool] Edit (via Go): {path}")
        result = await client.execute_tool(current_session_id(), "edit", {
            "path": path,
            "old_string": old_string,
            "new_string": new_string,
        })
        return result["result"]

    return edit


def make_trash_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 trash 工具 handler（移入回收站，可恢复）。"""

    async def trash(path: str = "", paths: list | None = None) -> str:
        args = {"paths": paths} if paths else {"path": path}
        logging.info(f"[Tool] Trash (via Go): {args}")
        result = await client.execute_tool(current_session_id(), "trash", args)
        return _format_result(result)

    return trash


def make_purge_handler(client: "FlowPartnerClient"):
    """创建通过 Go 执行的 purge 工具 handler（永久删除，需显式审批）。"""

    async def purge(entry: str = "") -> str:
        args = {"entry": entry} if entry else {}
        logging.info(f"[Tool] Purge (via Go): {args}")
        result = await client.execute_tool(current_session_id(), "purge", args)
        return _format_result(result)

    return purge
