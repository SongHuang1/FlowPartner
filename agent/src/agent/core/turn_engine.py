"""TurnEngine：回合生命周期管理。

四分支：prepare → sample loop → finalize / abort。
采样循环：drain steering → assemble prompt → call_llm → ParallelToolRuntime → check continuation。
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import logging
import time
import uuid
from dataclasses import dataclass, field
from typing import Any, Awaitable, Callable, Optional

from agent.core.constants import LOOP_DEADLINE_SECONDS, MAX_ITERATIONS, MAX_TOOL_RESULT_CHARS, STUCK_THRESHOLD
from agent.core.steer import SteerInput, TurnContext
from agent.core.tool_runtime import ParallelToolRuntime, ToolCall, ToolResult, WRITE_TOOLS

logger = logging.getLogger(__name__)


@dataclass
class StepContext:
    """回合级冻结上下文。

    同一回合内多次采样使用的 system_prompt 与工具清单必须完全一致。
    """
    system_prompt: str
    tools_definition: list[dict[str, Any]]
    system_prompt_hash: str = ""
    tools_hash: str = ""

    def __post_init__(self) -> None:
        self.system_prompt_hash = self._hash(self.system_prompt)
        self.tools_hash = self._hash(json.dumps(self.tools_definition, sort_keys=True))

    @staticmethod
    def _hash(content: str) -> str:
        return hashlib.sha256(content.encode("utf-8")).hexdigest()[:16]

    def assert_frozen(self, other: StepContext) -> None:
        """验证跨采样的一致性。"""
        if self.system_prompt_hash != other.system_prompt_hash:
            logger.warning("System prompt hash changed mid-turn!")
        if self.tools_hash != other.tools_hash:
            logger.warning("Tools hash changed mid-turn!")


@dataclass
class TurnResult:
    """回合执行结果。"""
    last_agent_message: str
    duration_ms: int
    aborted: bool = False
    abort_reason: str = ""


@dataclass
class _LLMStreamResult:
    """LLM 流消费结果。"""
    content: str
    tool_calls: list[ToolCall]
    finish_reason: str
    usage: dict[str, Any] = field(default_factory=dict)


# Type aliases
CallLLMFunc = Callable[[list[dict[str, Any]], list[dict[str, Any]], Optional[str]], Awaitable[_LLMStreamResult]]
ExecuteToolFunc = Callable[[str, dict[str, Any]], Awaitable[dict[str, Any]]]
EmitEventFunc = Callable[[str, dict[str, Any]], Awaitable[None]]


class TurnEngine:
    """回合引擎：管理单个回合的完整生命周期。

    使用方式：
        engine = TurnEngine(...)
        result = await engine.run(user_message, history)
    """

    def __init__(
        self,
        session_id: str,
        thread_id: str,
        call_llm_func: CallLLMFunc,
        execute_tool_func: ExecuteToolFunc,
        emit_event_func: EmitEventFunc,
        system_prompt: str,
        tools_definition: list[dict[str, Any]],
        model_context_window: int = 0,
        turn_ctx: Optional[TurnContext] = None,
    ) -> None:
        self.session_id = session_id
        self.thread_id = thread_id
        self.turn_id = uuid.uuid4().hex[:12]
        self._call_llm = call_llm_func
        self._execute_tool = execute_tool_func
        self._emit_event = emit_event_func
        self._model_context_window = model_context_window

        self._step_ctx = StepContext(
            system_prompt=system_prompt,
            tools_definition=tools_definition,
        )
        if turn_ctx is not None:
            self._turn_ctx = turn_ctx
            self._turn_ctx.turn_id = self.turn_id
        else:
            self._turn_ctx = TurnContext(
                thread_id=thread_id,
                turn_id=self.turn_id,
            )

        self._messages: list[dict[str, Any]] = []
        self._total_input_tokens = 0
        self._total_output_tokens = 0
        self._total_cached_tokens = 0
        self._usage_estimated = False

    async def run(
        self,
        user_message: str,
        history: Optional[list[dict[str, Any]]] = None,
    ) -> TurnResult:
        start_time = time.monotonic()

        await self._emit_event("turn_started", {
            "thread_id": self.thread_id,
            "turn_id": self.turn_id,
        })

        self._messages = list(history) if history else []
        self._messages.append({"role": "user", "content": user_message})

        try:
            await self._sample_loop()
        except asyncio.CancelledError:
            duration_ms = int((time.monotonic() - start_time) * 1000)
            await self._emit_usage()
            await self._emit_event("turn_aborted", {
                "thread_id": self.thread_id,
                "turn_id": self.turn_id,
                "reason": "cancelled",
            })
            return TurnResult(
                last_agent_message="",
                duration_ms=duration_ms,
                aborted=True,
                abort_reason="cancelled",
            )

        duration_ms = int((time.monotonic() - start_time) * 1000)

        if self._turn_ctx.is_interrupted():
            await self._emit_usage()
            await self._emit_event("turn_aborted", {
                "thread_id": self.thread_id,
                "turn_id": self.turn_id,
                "reason": "interrupted",
            })
            return TurnResult(
                last_agent_message="",
                duration_ms=duration_ms,
                aborted=True,
                abort_reason="interrupted",
            )

        await self._emit_usage()
        last_agent_message = self._get_last_agent_message()
        await self._emit_event("turn_completed", {
            "thread_id": self.thread_id,
            "turn_id": self.turn_id,
            "last_agent_message": last_agent_message,
            "duration_ms": duration_ms,
        })

        return TurnResult(
            last_agent_message=last_agent_message,
            duration_ms=duration_ms,
        )

    async def _sample_loop(self) -> None:
        iteration = 0
        start_time = time.monotonic()
        tool_call_signatures: list[str] = []

        while True:
            iteration += 1

            if self._turn_ctx.is_interrupted():
                return

            # ① 在采样边界安全消费 steering 队列
            self._drain_steering()

            # ② 检查循环终止条件
            termination = self._check_loop_termination(
                iteration, start_time, tool_call_signatures
            )
            if termination:
                logger.info("Loop terminated: %s", termination)
                return

            # ③ 组装 Prompt 并调用 LLM
            stream_result = await self._call_llm_stream()

            # ④ 累加 usage
            self._accumulate_usage(stream_result.usage)

            # ⑤ 无工具调用 → 本轮结束
            if not stream_result.tool_calls:
                self._messages.append({
                    "role": "assistant",
                    "content": stream_result.content,
                })
                if stream_result.finish_reason == "stop":
                    return
                continue

            # ⑥ 记录 assistant message with tool_calls
            assistant_msg: dict[str, Any] = {
                "role": "assistant",
                "content": stream_result.content,
                "tool_calls": [
                    {
                        "id": tc.call_id,
                        "type": "function",
                        "function": {
                            "name": tc.tool_name,
                            "arguments": json.dumps(tc.arguments, ensure_ascii=False),
                        },
                    }
                    for tc in stream_result.tool_calls
                ],
            }
            self._messages.append(assistant_msg)

            # ⑦ 并发执行工具
            tool_results = await self._execute_tools(stream_result.tool_calls)

            # ⑧ 将工具结果写入历史
            for tc, tr in zip(stream_result.tool_calls, tool_results):
                result_text = tr.result
                if len(result_text) > MAX_TOOL_RESULT_CHARS:
                    result_text = result_text[:MAX_TOOL_RESULT_CHARS] + "\n... (结果已截断)"
                self._messages.append({
                    "role": "tool",
                    "tool_call_id": tc.call_id,
                    "name": tc.tool_name,
                    "content": result_text,
                })

            # ⑨ 记录 tool call signatures 用于 stuck 检测
            for tc in stream_result.tool_calls:
                sig = f"{tc.tool_name}:{json.dumps(tc.arguments, sort_keys=True)}"
                tool_call_signatures.append(sig)

            # ⑩ 续轮判定：steering 队列非空 → 继续
            if not self._turn_ctx.steer_queue.empty():
                continue

    def _drain_steering(self) -> None:
        """在采样边界 drain steering 队列，将插话作为 user 消息追加。"""
        steers = self._turn_ctx.steer_queue.drain()
        for steer in steers:
            self._messages.append({
                "role": "user",
                "content": steer.content,
            })

    async def _call_llm_stream(self) -> _LLMStreamResult:
        """调用 LLM 并流式消费结果。"""
        result = await self._call_llm(
            messages=self._messages,
            tools=self._step_ctx.tools_definition,
        )
        return result

    async def _execute_tools(self, tool_calls: list[ToolCall]) -> list[ToolResult]:
        """并发执行工具调用。"""
        runtime = ParallelToolRuntime(
            execute_func=self._execute_tool,
            turn_ctx=self._turn_ctx,
        )

        item_ids: dict[str, str] = {}
        for tc in tool_calls:
            item_id = uuid.uuid4().hex[:8]
            item_ids[tc.call_id] = item_id
            item_type = self._map_tool_to_item_type(tc.tool_name)
            await self._emit_event("item_started", {
                "item_id": item_id,
                "item_type": item_type,
                "thread_id": self.thread_id,
                "turn_id": self.turn_id,
            })

        results = await runtime.run(tool_calls)

        for tc, tr in zip(tool_calls, results):
            item_id = item_ids[tc.call_id]
            item_type = self._map_tool_to_item_type(tc.tool_name)
            await self._emit_event("item_completed", {
                "item_id": item_id,
                "item_type": item_type,
                "thread_id": self.thread_id,
                "turn_id": self.turn_id,
                "payload": json.dumps({
                    "success": tr.success,
                    "result": tr.result,
                    "error_code": tr.error_code,
                }, ensure_ascii=False),
            })

        return results

    @staticmethod
    def _map_tool_to_item_type(tool_name: str) -> str:
        """映射工具名称到 item type（规范 FR-E7）。"""
        if tool_name in WRITE_TOOLS:
            return "patchApply"
        if tool_name == "read":
            return "commandExecution"
        if tool_name == "bash":
            return "commandExecution"
        return "commandExecution"

    def _accumulate_usage(self, usage: dict[str, Any]) -> None:
        """累加 token 用量。"""
        if not usage:
            return
        self._total_input_tokens += usage.get("input_tokens", 0)
        self._total_output_tokens += usage.get("output_tokens", 0)
        self._total_cached_tokens += usage.get("cached_input_tokens", 0)
        if usage.get("estimated", False):
            self._usage_estimated = True

    async def _emit_usage(self) -> None:
        """发送 usage_update 事件。"""
        total = self._total_input_tokens + self._total_output_tokens
        await self._emit_event("usage_update", {
            "input_tokens": self._total_input_tokens,
            "cached_input_tokens": self._total_cached_tokens,
            "output_tokens": self._total_output_tokens,
            "total_tokens": total,
            "model_context_window": self._model_context_window,
            "estimated": self._usage_estimated,
        })

    def _check_loop_termination(
        self,
        iteration: int,
        start_time: float,
        tool_call_signatures: list[str],
    ) -> Optional[str]:
        """检查循环终止条件。返回终止原因或 None。"""
        if iteration >= MAX_ITERATIONS:
            return f"max_iterations ({MAX_ITERATIONS})"

        elapsed = time.monotonic() - start_time
        if elapsed >= LOOP_DEADLINE_SECONDS:
            return f"time_budget ({LOOP_DEADLINE_SECONDS}s)"

        if len(tool_call_signatures) >= STUCK_THRESHOLD:
            last = tool_call_signatures[-STUCK_THRESHOLD:]
            # 检测连续重复（同一工具调用重复 N 次）
            if len(set(last)) == 1:
                return f"stuck (repeated {STUCK_THRESHOLD}x)"
            # 检测交替循环（如 A→B→A→B 模式）
            if len(last) >= 4 and len(set(last)) == 2:
                # 检查是否为 ABAB 模式
                if last[0] == last[2] and last[1] == last[3] and last[0] != last[1]:
                    return f"stuck (alternating {STUCK_THRESHOLD}x)"

        return None

    def _get_last_agent_message(self) -> str:
        """获取最后一条 assistant 消息的内容。"""
        for msg in reversed(self._messages):
            if msg.get("role") == "assistant" and msg.get("content"):
                return msg["content"]
        return ""
