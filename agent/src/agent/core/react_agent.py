import asyncio
import json
import logging
import time
import uuid
from typing import Any


MAX_ITERATIONS = 15
LOOP_DEADLINE_SECONDS = 5 * 60
TOKEN_BUDGET = 60000000  # 累计 token 上限（输入+输出合计）
STUCK_THRESHOLD = 3  # 连续相同工具调用触发卡死

# 默认系统提示词（与 Go 侧 defaultMainPrompt 一致）；executor 定义缺失时的兜底
DEFAULT_SYSTEM_PROMPT = (
    "你是一个强大的本地 AI 助手。你可以使用工具读取文件、写入文件、浏览目录等，"
    "帮助用户完成各种任务。请根据用户需求合理使用工具。删除任何文件或目录时，"
    "必须使用 trash 工具（移入回收站，可恢复），禁止使用 shell 删除命令。"
    "仅当用户明确要求永久删除回收站内容时，才使用 purge 工具（该操作不可逆且每次都需要用户审批）。"
)


def _make_tool_call_signature(tool_call: dict) -> str:
    """生成工具调用的签名（tool_name + arguments），用于重复检测。"""
    func = tool_call.get("function", {})
    name = func.get("name", "")
    args = func.get("arguments", "")
    return f"{name}:{args}"


class ReactAgent:
    """ReAct Agent 核心：思考 -> 行动 -> 观察 -> 循环（健壮版）"""

    def __init__(self, session_id: str, call_llm_func, send_event_func, tool_registry):
        self.session_id = session_id
        self.call_llm = call_llm_func
        self.send_event = send_event_func
        self.tools = tool_registry

    async def _emit(self, event_type: str, payload: dict) -> None:
        """发送事件到前端。"""
        await self.send_event(self.session_id, event_type, payload)

    def _check_loop_termination(
        self,
        iteration: int,
        start_time: float,
        total_tokens: int,
        tool_call_history: list[str],
    ) -> str | None:
        """检查循环护栏是否触发。返回 reason 或 None。"""
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
    ) -> tuple[str, list[dict]]:

        if history is None:
            history = []

        system_content = system_prompt if system_prompt else DEFAULT_SYSTEM_PROMPT

        messages: list[dict[str, Any]] = [
            {"role": "system", "content": system_content}
        ]

        for h in history:
            role = h.get("role", "")
            if role in ("user", "assistant"):
                msg: dict[str, Any] = {"role": role, "content": h.get("content", "")}
                if role == "assistant":
                    if h.get("tool_calls"):
                        msg["tool_calls"] = h["tool_calls"]
                messages.append(msg)
            elif role == "tool":
                msg = {"role": "tool", "content": h.get("content", "")}
                if h.get("tool_call_id"):
                    msg["tool_call_id"] = h["tool_call_id"]
                messages.append(msg)

        messages.append({"role": "user", "content": user_message})

        tools_def = self.tools.get_openai_tools_definition()
        start_time = time.monotonic()
        total_tokens = 0
        tool_call_history: list[str] = []

        user_msg_record = {"role": "user", "content": user_message}
        recorded_messages: list[dict[str, Any]] = [user_msg_record]

        if forced_tool_call:
            forced_name = forced_tool_call.get("name", "")
            forced_args = forced_tool_call.get("arguments", {})
            if not isinstance(forced_args, dict):
                forced_args = {}
            forced_call_id = f"call_forced_{uuid.uuid4().hex[:12]}"
            forced_call = {
                "id": forced_call_id,
                "type": "function",
                "function": {"name": forced_name, "arguments": json.dumps(forced_args, ensure_ascii=False)},
            }
            forced_assistant_msg: dict[str, Any] = {
                "role": "assistant",
                "content": "",
                "tool_calls": [forced_call],
            }
            messages.append(forced_assistant_msg)
            recorded_messages.append(forced_assistant_msg)

            sig = _make_tool_call_signature(forced_call)
            tool_call_history.append(sig)

            logging.info(f"[ReAct] forced tool_call: {forced_name}")
            await self._emit("tool_call", {
                "tool": forced_name,
                "args": forced_args,
                "call_id": forced_call_id,
                "iteration": 1,
            })

            forced_result = await self.tools.execute(forced_name, forced_args)
            truncated = False
            if len(forced_result) > 10000:
                forced_result = forced_result[:10000]
                truncated = True

            await self._emit("tool_result", {
                "tool": forced_name,
                "result": forced_result,
                "call_id": forced_call_id,
                "iteration": 1,
                "truncated": truncated,
            })

            forced_tool_msg = {
                "role": "tool",
                "tool_call_id": forced_call_id,
                "content": forced_result,
            }
            messages.append(forced_tool_msg)
            recorded_messages.append(forced_tool_msg)

        try:
            for iteration in range(MAX_ITERATIONS):
                logging.info(f"[ReAct] iteration={iteration + 1}/{MAX_ITERATIONS} session={self.session_id}")

                await self._emit("iteration_start", {
                    "iteration": iteration + 1,
                    "max_iterations": MAX_ITERATIONS,
                })

                payload = json.dumps({
                    "model": "auto",
                    "messages": messages,
                    "tools": tools_def if tools_def else None,
                }, ensure_ascii=False)

                llm_resp = await self.call_llm(
                    self.session_id, payload,
                    send_event_func=send_event_func,
                    iteration=iteration + 1,
                )

                if not llm_resp.get("success"):
                    error_msg = llm_resp.get("error_message", "未知错误")
                    error_guess = llm_resp.get("error_guess", "")
                    event_payload: dict[str, Any] = {"message": error_msg}
                    if error_guess:
                        event_payload["guess"] = error_guess
                    await self._emit("error", event_payload)
                    return error_msg, recorded_messages

                if llm_resp.get("usage"):
                    usage = llm_resp["usage"]
                    total_tokens += usage.get("prompt_tokens", 0) + usage.get("completion_tokens", 0)

                finish_reason = llm_resp.get("finish_reason", "")

                term_reason = self._check_loop_termination(
                    iteration + 1, start_time, total_tokens, tool_call_history
                )
                if term_reason:
                    logging.warning(f"[ReAct] Loop terminated: reason={term_reason} session={self.session_id}")
                    await self._emit("loop_terminated", {
                        "reason": term_reason,
                        "iteration": iteration + 1,
                    })

                    summary = self._termination_summary(term_reason)
                    await self._emit("final_answer", {
                        "text": summary,
                        "iteration": iteration + 1,
                    })
                    recorded_messages.append({"role": "assistant", "content": summary})
                    return summary, recorded_messages

                if finish_reason == "tool_calls" and llm_resp.get("tool_calls"):
                    assistant_msg: dict[str, Any] = {
                        "role": "assistant",
                        "content": llm_resp.get("content", ""),
                        "tool_calls": llm_resp["tool_calls"],
                    }
                    messages.append(assistant_msg)
                    recorded_messages.append(assistant_msg)

                    for tool_call in llm_resp["tool_calls"]:
                        func_name = tool_call["function"]["name"]
                        try:
                            func_args = json.loads(tool_call["function"]["arguments"])
                        except json.JSONDecodeError:
                            logging.warning(f"[ReAct] Tool {func_name} called with malformed JSON arguments, using empty args")
                            func_args = {}
                        call_id = tool_call.get("id", "")

                        sig = _make_tool_call_signature(tool_call)
                        tool_call_history.append(sig)

                        logging.info(f"[ReAct] tool_call: {func_name} call_id={call_id}")
                        await self._emit("tool_call", {
                            "tool": func_name,
                            "args": func_args,
                            "call_id": call_id,
                            "iteration": iteration + 1,
                        })

                        result = await self.tools.execute(func_name, func_args)

                        truncated = False
                        if len(result) > 10000:
                            result = result[:10000]
                            truncated = True

                        await self._emit("tool_result", {
                            "tool": func_name,
                            "result": result,
                            "call_id": call_id,
                            "iteration": iteration + 1,
                            "truncated": truncated,
                        })

                        tool_msg = {
                            "role": "tool",
                            "tool_call_id": call_id,
                            "content": result,
                        }
                        messages.append(tool_msg)
                        recorded_messages.append(tool_msg)

                    continue

                final_answer = llm_resp.get("content", "")
                if finish_reason == "stop" or final_answer:
                    logging.info(f"[ReAct] final_answer length={len(final_answer)}")
                    await self._emit("final_answer", {
                        "text": final_answer,
                        "iteration": iteration + 1,
                    })
                    recorded_messages.append({"role": "assistant", "content": final_answer})
                    return final_answer, recorded_messages

                if not final_answer and not finish_reason:
                    await self._emit("final_answer", {
                        "text": final_answer or "",
                        "iteration": iteration + 1,
                        "incomplete": True,
                    })
                    recorded_messages.append({"role": "assistant", "content": final_answer or ""})
                    return final_answer or "", recorded_messages

            fallback = "抱歉，经过多轮尝试后未能得出结论。请尝试简化你的问题。"
            await self._emit("final_answer", {
                "text": fallback,
                "iteration": MAX_ITERATIONS,
            })
            recorded_messages.append({"role": "assistant", "content": fallback})
            return fallback, recorded_messages

        except asyncio.CancelledError:
            logging.info(f"[ReAct] Cancelled during execution: session={self.session_id}")
            try:
                await self._emit("final_answer", {
                    "text": _CANCELLED_ANSWER,
                    "cancelled": True,
                })
            except Exception:
                logging.warning(f"[ReAct] Failed to send cancelled event: session={self.session_id}")
            raise

    @staticmethod
    def _termination_summary(reason: str) -> str:
        summaries = {
            "max_iterations": f"已达最大迭代次数上限（{MAX_ITERATIONS} 轮），无法继续执行。请尝试简化你的问题或分步提问。",
            "time_budget": f"已超过整轮执行时间上限（{LOOP_DEADLINE_SECONDS // 60} 分钟），为防止资源浪费已终止。",
            "token_budget": f"已超过累计 token 上限（{TOKEN_BUDGET}），为防止资源浪费已终止。",
            "stuck": "检测到连续重复的工具调用，Agent 可能陷入死循环，已终止。",
        }
        return summaries.get(reason, f"循环因未知原因终止：{reason}")


_CANCELLED_ANSWER = "已停止生成"
