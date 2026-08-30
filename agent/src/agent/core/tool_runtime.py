from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass, field
from typing import Any, Callable, Awaitable

from agent.core.steer import TurnContext

logger = logging.getLogger(__name__)


WRITE_TOOLS: set[str] = {"write", "edit", "apply_patch", "trash", "purge"}
MAX_CONCURRENCY = 4


@dataclass
class ToolCall:
    call_id: str
    tool_name: str
    arguments: dict[str, Any]


@dataclass
class ToolResult:
    call_id: str
    tool_name: str
    success: bool
    result: str
    error_code: str = ""


@dataclass
class _PendingTool:
    index: int
    call: ToolCall
    future: asyncio.Future[ToolResult] = field(default_factory=lambda: asyncio.Future())


class ParallelToolRuntime:

    def __init__(
        self,
        execute_func: Callable[[str, dict[str, Any]], Awaitable[dict[str, Any]]],
        turn_ctx: TurnContext,
    ) -> None:
        self._execute_func = execute_func
        self._turn_ctx = turn_ctx
        self._global_sem = asyncio.Semaphore(MAX_CONCURRENCY)
        self._write_sem = asyncio.Semaphore(1)

    async def run(self, tools: list[ToolCall]) -> list[ToolResult]:
        if not tools:
            return []

        pending = [_PendingTool(index=i, call=tool) for i, tool in enumerate(tools)]

        for p in pending:
            p.future = asyncio.create_task(self._execute_one(p))

        interrupt_task = asyncio.create_task(self._watch_interrupt(pending))

        results: list[ToolResult] = []
        try:
            for p in pending:
                try:
                    result = await p.future
                    results.append(result)
                except asyncio.CancelledError:
                    results.append(ToolResult(
                        call_id=p.call.call_id,
                        tool_name=p.call.tool_name,
                        success=False,
                        result="工具执行已被中断",
                        error_code="CANCELLED",
                    ))
                except Exception as e:
                    logger.exception("Tool execution unexpected error: %s", p.call.call_id)
                    results.append(ToolResult(
                        call_id=p.call.call_id,
                        tool_name=p.call.tool_name,
                        success=False,
                        result=f"工具执行异常: {e}",
                        error_code="TOOL_ERROR",
                    ))
        finally:
            interrupt_task.cancel()

        return results

    async def _watch_interrupt(self, pending: list[_PendingTool]) -> None:
        await self._turn_ctx.interrupt_event.wait()
        for p in pending:
            if not p.future.done():
                p.future.cancel()

    async def _execute_one(self, pending: _PendingTool) -> ToolResult:
        call = pending.call
        is_write = call.tool_name in WRITE_TOOLS

        try:
            if is_write:
                async with self._write_sem:
                    async with self._global_sem:
                        return await self._do_execute(call)
            else:
                async with self._global_sem:
                    return await self._do_execute(call)
        except asyncio.CancelledError:
            raise
        except Exception as e:
            logger.exception("Tool execution failed: %s", call.call_id)
            return ToolResult(
                call_id=call.call_id,
                tool_name=call.tool_name,
                success=False,
                result=f"工具执行失败: {e}",
                error_code="TOOL_ERROR",
            )

    async def _do_execute(self, call: ToolCall) -> ToolResult:
        if self._turn_ctx.is_interrupted():
            return ToolResult(
                call_id=call.call_id,
                tool_name=call.tool_name,
                success=False,
                result="工具执行已被中断",
                error_code="CANCELLED",
            )

        try:
            raw = await self._execute_func(call.tool_name, call.arguments)
            return ToolResult(
                call_id=call.call_id,
                tool_name=call.tool_name,
                success=raw.get("success", False),
                result=raw.get("result", ""),
                error_code=raw.get("error_code", ""),
            )
        except asyncio.CancelledError:
            raise
        except Exception as e:
            return ToolResult(
                call_id=call.call_id,
                tool_name=call.tool_name,
                success=False,
                result=f"工具执行失败: {e}",
                error_code="TOOL_ERROR",
            )
