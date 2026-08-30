from __future__ import annotations

import asyncio
import json
import logging
import time
import uuid
from typing import Any

from agent.core.turn_engine import TurnEngine, _LLMStreamResult
from agent.core.tool_runtime import ToolCall

logger = logging.getLogger(__name__)

MAX_ITERATIONS = 15
LOOP_DEADLINE_SECONDS = 5 * 60
TOKEN_BUDGET = 60000000
STUCK_THRESHOLD = 3

DEFAULT_SYSTEM_PROMPT = (
    "你是一个强大的本地 AI 助手。你可以使用工具读取文件、写入文件、浏览目录等，"
    "帮助用户完成各种任务。请根据用户需求合理使用工具。删除任何文件或目录时，"
    "必须使用 trash 工具（移入回收站，可恢复），禁止使用 shell 删除命令。"
    "仅当用户明确要求永久删除回收站内容时，才使用 purge 工具（该操作不可逆且每次都需要用户审批）。"
)

_CANCELLED_ANSWER = "已停止生成"


def _make_tool_call_signature(tool_call: dict) -> str:
    func = tool_call.get("function", {})
    name = func.get("name", "")
    args = func.get("arguments", "")
    return f"{name}:{args}"


class ReactAgent:
    """ReAct Agent 兼容入口。

    内部创建 TurnEngine 执行回合，通过 send_event 回调发射旧格式事件
    （供 SubAgentRunner._forward 转换）。
    """

    def __init__(self, session_id: str, call_llm_func, send_event_func, tool_registry):
        self.session_id = session_id
        self.call_llm = call_llm_func
        self.send_event = send_event_func
        self.tools = tool_registry

    async def _emit(self, event_type: str, payload: dict) -> None:
        await self.send_event(self.session_id, event_type, payload)

    def _check_loop_termination(
        self,
        iteration: int,
        start_time: float,
        total_tokens: int,
        tool_call_history: list[str],
    ) -> str | None:
        if iteration >= MAX_ITERATIONS:
            return "max_iterations"
        elapsed = time.monotonic() - start_time
        if elapsed > LOOP_DEADLINE_SECONDS:
            return "time_budget"
        if total_tokens > TOKEN_BUDGET:
            return "token_budget"
        if len(tool_call_history) >= STUCK_THRESHOLD:
            last_n = tool_call_history[-STUCK_THRESHOLD:]
            if len(set(last_n)) == 1 and last_n[0] != "":
                return "stuck"
        return None

    async def run(
        self,
        user_message: str,
        history: list | None = None,
        send_event_func=None,
        system_prompt: str | None = None,
        forced_tool_call: dict | None = None,
        turn_ctx_factory=None,
    ) -> tuple[str, list[dict]]:
        system_content = system_prompt if system_prompt else DEFAULT_SYSTEM_PROMPT
        tools_def = self.tools.get_openai_tools_definition()

        async def call_llm_adapter(
            messages: list[dict[str, Any]],
            tools: list[dict[str, Any]],
            forced_name: str | None = None,
        ) -> _LLMStreamResult:
            # 系统提示词作为第一条消息（StepContext 冻结）
            full_messages = [{"role": "system", "content": system_content}] + messages
            payload_dict: dict[str, Any] = {
                "model": "auto",
                "messages": full_messages,
                "tools": tools if tools else None,
            }
            if forced_name:
                payload_dict["tool_choice"] = {"type": "function", "function": {"name": forced_name}}

            payload_str = json.dumps(payload_dict, ensure_ascii=False)
            result = await self.call_llm(self.session_id, payload_str, send_event_func=send_event_func)

            if not result.get("success"):

                await self._emit("error", {
                    "message": result.get("error_message", "未知错误"),
                    "guess": result.get("error_guess", ""),
                })

                return _LLMStreamResult(
                    content=result.get("error_message", ""),
                    tool_calls=[],
                    finish_reason="stop",
                )

            tool_calls = []
            for tc in result.get("tool_calls", []):
                func = tc.get("function", {})
                args_str = func.get("arguments", "{}")
                try:
                    args = json.loads(args_str)
                except json.JSONDecodeError:
                    # 参数 JSON 非法时丢弃该工具调用（与 grpc_client 服务端重组策略一致）
                    logger.warning(
                        f"[LLM] Tool call {func.get('name', '?')} has invalid JSON arguments "
                        f"({args_str[:100]}...), dropping"
                    )
                    continue
                tool_calls.append(ToolCall(
                    call_id=tc.get("id", ""),
                    tool_name=func.get("name", ""),
                    arguments=args,
                ))

            return _LLMStreamResult(
                content=result.get("content", ""),
                tool_calls=tool_calls,
                finish_reason=result.get("finish_reason", ""),
                usage=result.get("usage", {}),
            )

        async def execute_tool_adapter(tool_name: str, arguments: dict) -> dict:
            try:
                result = await self.tools.execute(tool_name, arguments)
                return {"success": True, "result": result, "error_code": ""}
            except Exception as e:
                return {"success": False, "result": f"工具执行失败: {e}", "error_code": "TOOL_ERROR"}

        async def emit_adapter(event_type: str, payload: dict) -> None:
            await self._map_and_emit(event_type, payload)

        # 创建或复用 TurnContext
        turn_ctx = None
        if turn_ctx_factory is not None:
            turn_ctx = turn_ctx_factory()

        engine = TurnEngine(
            session_id=self.session_id,
            thread_id=self.session_id,
            call_llm_func=call_llm_adapter,
            execute_tool_func=execute_tool_adapter,
            emit_event_func=emit_adapter,
            system_prompt=system_content,
            tools_definition=tools_def,
            turn_ctx=turn_ctx,
        )


        original_history_len = len(history) if history else 0

        # 处理 forced_tool_call
        if forced_tool_call:
            forced_name = forced_tool_call.get("name", "")
            forced_args = forced_tool_call.get("arguments", {})
            if not isinstance(forced_args, dict):
                forced_args = {}
            if history is None:
                history = []
            forced_call_id = f"call_forced_{uuid.uuid4().hex[:12]}"
            history.append({
                "role": "assistant",
                "content": "",
                "tool_calls": [{
                    "id": forced_call_id,
                    "type": "function",
                    "function": {"name": forced_name, "arguments": json.dumps(forced_args, ensure_ascii=False)},
                }],
            })
            tool_result = await execute_tool_adapter(forced_name, forced_args)
            result_text = tool_result.get("result", "")
            truncated = len(result_text) > 10000
            if truncated:
                result_text = result_text[:10000]
            history.append({
                "role": "tool",
                "tool_call_id": forced_call_id,
                "name": forced_name,
                "content": result_text,
            })
            await self._emit("tool_call", {
                "tool": forced_name,
                "args": forced_args,
                "call_id": forced_call_id,
                "iteration": 1,
            })
            await self._emit("tool_result", {
                "tool": forced_name,
                "result": result_text,
                "call_id": forced_call_id,
                "iteration": 1,
                "truncated": truncated,
            })

        turn_result = await engine.run(user_message, history=history)

        recorded_messages = self._build_recorded_messages(
            original_history_len, user_message, engine._messages,
        )

        if turn_result.aborted:
            await self._emit("final_answer", {"text": _CANCELLED_ANSWER, "cancelled": True})
            return _CANCELLED_ANSWER, recorded_messages

        await self._emit("final_answer", {
            "text": turn_result.last_agent_message,
        })
        return turn_result.last_agent_message, recorded_messages

    async def _map_and_emit(self, event_type: str, payload: dict) -> None:
        """透传 TurnEngine 新事件，同时附加旧格式事件（向后兼容）。"""
        # 透传所有新事件
        await self._emit(event_type, payload)

        # 附加旧格式事件（供 SubAgentRunner 等下游消费）
        if event_type == "turn_started":
            await self._emit("iteration_start", {
                "iteration": 1,
                "max_iterations": MAX_ITERATIONS,
            })
        elif event_type == "turn_aborted":
            await self._emit("loop_terminated", {
                "reason": "interrupted",
                "iteration": 1,
            })

    def _build_recorded_messages(
        self,
        original_history_len: int,
        user_message: str,
        engine_messages: list[dict],
    ) -> list[dict]:
        recorded: list[dict] = [{"role": "user", "content": user_message}]
        # 只记录本轮新增消息（排除原始历史消息，但包含 forced_tool_call 消息）
        new_messages = engine_messages[original_history_len:]
        for msg in new_messages:
            role = msg.get("role", "")
            if role in ("assistant", "tool"):
                recorded.append(msg)
        return recorded

    @staticmethod
    def _termination_summary(reason: str) -> str:
        summaries = {
            "max_iterations": f"已达最大迭代次数上限（{MAX_ITERATIONS} 轮），无法继续执行。请尝试简化你的问题或分步提问。",
            "time_budget": f"已超过整轮执行时间上限（{LOOP_DEADLINE_SECONDS // 60} 分钟），为防止资源浪费已终止。",
            "token_budget": f"已超过累计 token 上限（{TOKEN_BUDGET}），为防止资源浪费已终止。",
            "stuck": "检测到连续重复的工具调用，Agent 可能陷入死循环，已终止。",
        }
        return summaries.get(reason, f"循环因未知原因终止：{reason}")
