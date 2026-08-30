import asyncio
import json
from unittest.mock import AsyncMock

import pytest

from agent.core.react_agent import MAX_ITERATIONS, ReactAgent
from agent.core.tool_runtime import ToolCall
from agent.tools.registry import ToolRegistry


def make_agent(call_llm_func, tool_registry=None):
    if tool_registry is None:
        tool_registry = ToolRegistry()
    send_event = AsyncMock()
    return ReactAgent(
        session_id="test-session",
        call_llm_func=call_llm_func,
        send_event_func=send_event,
        tool_registry=tool_registry,
    ), send_event


def get_events(send_event, event_type: str) -> list[dict]:
    """获取指定类型的所有事件 payload 列表。"""
    return [call.args[2] for call in send_event.await_args_list if call.args[1] == event_type]


def get_item_results(send_event) -> list[dict]:
    """从 item_completed 事件中提取解析后的 payload。"""
    results = []
    for payload in get_events(send_event, "item_completed"):
        try:
            results.append(json.loads(payload.get("payload", "{}")))
        except (json.JSONDecodeError, TypeError):
            results.append({})
    return results


class TestReactAgent:
    async def _run_simple_chat(self, llm_response):
        call_llm = AsyncMock(return_value=llm_response)
        agent, send_event = make_agent(call_llm)
        result, _ = await agent.run("Hello")
        return result, send_event, call_llm

    @pytest.mark.asyncio
    async def test_simple_response(self):
        """无工具调用：turn_started → final_answer。"""
        result, send_event, _ = await self._run_simple_chat({
            "success": True,
            "content": "Hi there!",
            "finish_reason": "stop",
        })
        assert result == "Hi there!"
        # turn_started 映射为 iteration_start
        iteration_events = get_events(send_event, "iteration_start")
        assert len(iteration_events) == 1
        assert iteration_events[0] == {"iteration": 1, "max_iterations": MAX_ITERATIONS}
        # final_answer
        final_events = get_events(send_event, "final_answer")
        assert len(final_events) == 1
        assert final_events[0]["text"] == "Hi there!"

    @pytest.mark.asyncio
    async def test_llm_error(self):
        """LLM 返回 success=False 时发 error 事件。"""
        call_llm = AsyncMock(return_value={
            "success": False,
            "error_message": "API key invalid",
            "error_guess": "Check your API key",
        })
        agent, send_event = make_agent(call_llm)
        result, _ = await agent.run("Hello")

        assert result == "API key invalid"
        error_events = get_events(send_event, "error")
        assert len(error_events) == 1
        assert error_events[0] == {"message": "API key invalid", "guess": "Check your API key"}

    @pytest.mark.asyncio
    async def test_tool_call_then_final(self):
        """工具调用→结果→续轮→final。事件类型为 item_started/item_completed。"""
        call_llm = AsyncMock(side_effect=[
            {
                "success": True,
                "content": "",
                "finish_reason": "tool_calls",
                "tool_calls": [
                    {
                        "id": "call_1",
                        "type": "function",
                        "function": {
                            "name": "echo",
                            "arguments": '{"text": "hello"}',
                        },
                    }
                ],
            },
            {
                "success": True,
                "content": "Done!",
                "finish_reason": "stop",
            },
        ])

        registry = ToolRegistry()

        async def echo_handler(text):
            return f"echo: {text}"

        registry.register("echo", "Echo", {}, echo_handler)

        agent, send_event = make_agent(call_llm, registry)
        result, _ = await agent.run("Use echo tool")

        assert result == "Done!"
        # item_started 事件
        started_events = get_events(send_event, "item_started")
        assert len(started_events) == 1
        assert started_events[0]["item_type"] == "commandExecution"
        # item_completed 事件（payload 为 JSON）
        completed_events = get_events(send_event, "item_completed")
        assert len(completed_events) == 1
        payload = json.loads(completed_events[0]["payload"])
        assert payload["success"] is True
        assert payload["result"] == "echo: hello"

    @pytest.mark.asyncio
    async def test_max_iterations_exceeded(self):
        """达到最大迭代次数时终止。"""
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "",
            "finish_reason": "length",
        })
        agent, send_event = make_agent(call_llm)
        result, _ = await agent.run("Loop forever")

        # 达到最大迭代后返回空内容（无 stop 原因的 assistant 消息）
        assert result == ""
        # LLM 调用次数 = MAX_ITERATIONS - 1（第 15 次迭代检测到终止条件后退出，不调用 LLM）
        assert call_llm.call_count == MAX_ITERATIONS - 1

    @pytest.mark.asyncio
    async def test_history_passed_through(self):
        """历史消息透传到 LLM payload。"""
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "Response",
            "finish_reason": "stop",
        })
        agent, _ = make_agent(call_llm)
        history = [{"role": "user", "content": "previous"}, {"role": "assistant", "content": "answer"}]
        await agent.run("New question", history=history)

        payload = call_llm.call_args[0][1]
        parsed = json.loads(payload)
        assert parsed["messages"][0]["role"] == "system"
        assert {"role": "user", "content": "previous"} in parsed["messages"]
        assert {"role": "assistant", "content": "answer"} in parsed["messages"]
        assert {"role": "user", "content": "New question"} in parsed["messages"]

    @pytest.mark.asyncio
    async def test_tools_definition_in_payload(self):
        """工具定义透传到 LLM payload。"""
        registry = ToolRegistry()
        registry.register("my_tool", "desc", {"type": "object"}, lambda: None)

        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "ok",
            "finish_reason": "stop",
        })
        agent, _ = make_agent(call_llm, registry)
        await agent.run("test")

        payload = json.loads(call_llm.call_args[0][1])
        assert payload["tools"] is not None
        assert len(payload["tools"]) == 1
        assert payload["tools"][0]["function"]["name"] == "my_tool"

    @pytest.mark.asyncio
    async def test_no_tools_when_registry_empty(self):
        """空 registry 时 tools=None。"""
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "ok",
            "finish_reason": "stop",
        })
        agent, _ = make_agent(call_llm)
        await agent.run("test")

        payload = json.loads(call_llm.call_args[0][1])
        assert payload["tools"] is None

    @pytest.mark.asyncio
    async def test_tool_call_malformed_json_arguments_recovers(self):
        """工具参数 JSON 非法时跳过该调用并恢复。"""
        call_llm = AsyncMock(side_effect=[
            {
                "success": True,
                "content": "",
                "finish_reason": "tool_calls",
                "tool_calls": [
                    {
                        "id": "call_1",
                        "type": "function",
                        "function": {"name": "echo", "arguments": "{broken json"},
                    }
                ],
            },
            {
                "success": True,
                "content": "recovered",
                "finish_reason": "stop",
            },
        ])

        registry = ToolRegistry()

        async def echo_handler(text):
            return f"echo: {text}"

        registry.register("echo", "Echo", {}, echo_handler)

        agent, send_event = make_agent(call_llm, registry)
        result, _ = await agent.run("Use tool")

        assert result == "recovered"
        # 非法参数的工具调用被丢弃，不产生 item_completed
        item_results = get_item_results(send_event)
        assert len(item_results) == 0

    @pytest.mark.asyncio
    async def test_tool_call_missing_id_recovers(self):
        """工具调用缺少 ID 时跳过并恢复。"""
        call_llm = AsyncMock(side_effect=[
            {
                "success": True,
                "content": "",
                "finish_reason": "tool_calls",
                "tool_calls": [
                    {
                        "type": "function",
                        "function": {"name": "echo", "arguments": '{"text": "hi"}'},
                    }
                ],
            },
            {
                "success": True,
                "content": "done",
                "finish_reason": "stop",
            },
        ])

        registry = ToolRegistry()

        async def echo_handler(text):
            return f"echo: {text}"

        registry.register("echo", "Echo", {}, echo_handler)

        agent, _ = make_agent(call_llm, registry)
        result, _ = await agent.run("Use tool")

        assert result == "done"

    @pytest.mark.asyncio
    async def test_multiple_tool_calls_in_single_response(self):
        """单次响应多个工具调用：并发执行，按调用顺序归位。"""
        call_llm = AsyncMock(side_effect=[
            {
                "success": True,
                "content": "",
                "finish_reason": "tool_calls",
                "tool_calls": [
                    {
                        "id": "c1",
                        "type": "function",
                        "function": {"name": "echo", "arguments": '{"text": "a"}'},
                    },
                    {
                        "id": "c2",
                        "type": "function",
                        "function": {"name": "echo", "arguments": '{"text": "b"}'},
                    },
                ],
            },
            {
                "success": True,
                "content": "done",
                "finish_reason": "stop",
            },
        ])

        registry = ToolRegistry()

        async def echo_handler(text):
            return f"echo: {text}"

        registry.register("echo", "Echo", {}, echo_handler)

        agent, send_event = make_agent(call_llm, registry)
        result, _ = await agent.run("Use tools")

        assert result == "done"
        item_results = get_item_results(send_event)
        assert len(item_results) == 2
        assert item_results[0]["result"] == "echo: a"
        assert item_results[1]["result"] == "echo: b"

    @pytest.mark.asyncio
    async def test_unknown_tool_result_fed_back(self):
        """未知工具调用返回错误消息作为 result。"""
        call_llm = AsyncMock(side_effect=[
            {
                "success": True,
                "content": "",
                "finish_reason": "tool_calls",
                "tool_calls": [
                    {
                        "id": "c1",
                        "type": "function",
                        "function": {"name": "nonexistent_tool", "arguments": "{}"},
                    }
                ],
            },
            {
                "success": True,
                "content": "ok",
                "finish_reason": "stop",
            },
        ])

        agent, send_event = make_agent(call_llm)
        result, _ = await agent.run("Use tool")

        assert result == "ok"
        item_results = get_item_results(send_event)
        assert len(item_results) == 1
        # 未知工具返回错误消息作为 result（不抛异常）
        assert "nonexistent_tool" in item_results[0]["result"]

    @pytest.mark.asyncio
    async def test_tool_result_truncated_to_10000_chars(self):
        """工具结果超过 10000 字符时截断（截断发生在消息写入阶段）。"""
        async def long_handler():
            return "x" * 12000

        registry = ToolRegistry()
        registry.register("long", "Long output", {}, long_handler)

        call_llm = AsyncMock(side_effect=[
            {
                "success": True,
                "content": "",
                "finish_reason": "tool_calls",
                "tool_calls": [
                    {
                        "id": "c1",
                        "type": "function",
                        "function": {"name": "long", "arguments": "{}"},
                    }
                ],
            },
            {
                "success": True,
                "content": "done",
                "finish_reason": "stop",
            },
        ])

        agent, send_event = make_agent(call_llm, registry)
        _, messages = await agent.run("Use tool")

        # 截断发生在消息写入阶段，检查 recorded_messages 中的 tool 消息
        tool_messages = [m for m in messages if m.get("role") == "tool"]
        assert len(tool_messages) == 1
        # 结果被截断为 10000 字符 + 截断标记
        assert len(tool_messages[0]["content"]) <= 10000 + 50  # 允许截断标记的长度
        assert tool_messages[0]["content"].startswith("x" * 100)

    @pytest.mark.asyncio
    async def test_send_event_func_forwarded_to_call_llm(self):
        """send_event_func 透传到 call_llm。"""
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "ok",
            "finish_reason": "stop",
        })
        agent, _ = make_agent(call_llm)

        llm_chunk_cb = AsyncMock()
        await agent.run("Hello", send_event_func=llm_chunk_cb)

        assert call_llm.await_args.kwargs.get("send_event_func") is llm_chunk_cb

    @pytest.mark.asyncio
    async def test_iteration_start_emitted_once(self):
        """turn_started 仅发出一次（映射为 iteration_start）。"""
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "",
            "finish_reason": "length",
        })
        agent, send_event = make_agent(call_llm)

        await agent.run("Loop")

        iteration_events = get_events(send_event, "iteration_start")
        assert len(iteration_events) == 1

    @pytest.mark.asyncio
    async def test_llm_error_without_guess(self):
        """LLM 错误无 guess 时不发 error 事件（旧兼容路径不再触发）。"""
        call_llm = AsyncMock(return_value={
            "success": False,
            "error_message": "boom",
        })
        agent, send_event = make_agent(call_llm)

        result, _ = await agent.run("Hello")

        assert result == "boom"
        error_events = get_events(send_event, "error")
        assert len(error_events) == 1
        assert error_events[0] == {"message": "boom", "guess": ""}

    @pytest.mark.asyncio
    async def test_forced_tool_call_executed_before_llm(self):
        """forced_tool_call 在 LLM 调用前执行。"""
        registry = ToolRegistry()

        async def echo_handler(task):
            return f"echo: {task}"

        registry.register("agent__sub-1", "Sub agent", {"type": "object"}, echo_handler)

        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "final",
            "finish_reason": "stop",
        })
        agent, send_event = make_agent(call_llm, registry)
        result, messages = await agent.run(
            "帮我完成任务",
            forced_tool_call={"name": "agent__sub-1", "arguments": {"task": "帮我完成任务"}},
        )

        assert result == "final"
        # 强制调用先执行并进入上下文
        forced_tool_msg = next(
            (m for m in messages if m.get("role") == "tool" and m.get("tool_call_id") == messages[1]["tool_calls"][0]["id"]),
            None,
        )
        assert forced_tool_msg is not None
        assert forced_tool_msg["content"] == "echo: 帮我完成任务"
        tool_calls = get_events(send_event, "tool_call")
        assert tool_calls[0]["tool"] == "agent__sub-1"

    @pytest.mark.asyncio
    async def test_usage_update_emitted_on_finalize(self):
        """回合结束时发出 usage_update 事件。"""
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "ok",
            "finish_reason": "stop",
            "usage": {
                "input_tokens": 100,
                "output_tokens": 50,
                "total_tokens": 150,
                "estimated": False,
            },
        })
        agent, send_event = make_agent(call_llm)
        await agent.run("Hello")

        usage_events = get_events(send_event, "usage_update")
        assert len(usage_events) == 1
        assert usage_events[0]["input_tokens"] == 100
        assert usage_events[0]["output_tokens"] == 50
        assert usage_events[0]["total_tokens"] == 150

    @pytest.mark.asyncio
    async def test_turn_aborted_emits_loop_terminated(self):
        """回合中断时 turn_aborted 映射为 loop_terminated。"""
        async def fake_call_llm(session_id, json_payload, send_event_func=None, iteration=None):
            raise asyncio.CancelledError()

        agent, send_event = make_agent(fake_call_llm)

        # CancelledError 被 TurnEngine 捕获，不向上传播
        result, _ = await agent.run("test message")
        assert result == "已停止生成"

        # turn_aborted 映射为 loop_terminated
        terminated_events = get_events(send_event, "loop_terminated")
        assert len(terminated_events) == 1
        assert terminated_events[0]["reason"] == "interrupted"
        # 同时发出 final_answer（cancelled）
        final_events = get_events(send_event, "final_answer")
        assert len(final_events) == 1
        assert "已停止" in final_events[0]["text"]


class TestReactAgentEdgeCases:
    @pytest.mark.asyncio
    async def test_cancelled_error_sends_final_answer(self):
        """react_agent 捕获 CancelledError 后应已发送 final_answer 事件。"""
        async def fake_call_llm(session_id, json_payload, send_event_func=None, iteration=None):
            raise asyncio.CancelledError()

        agent, send_event = make_agent(fake_call_llm)

        # CancelledError 被 TurnEngine 捕获，不向上传播
        result, _ = await agent.run("test message")
        assert result == "已停止生成"

        final_answer_calls = [c for c in send_event.await_args_list if c.args[1] == "final_answer"]
        assert len(final_answer_calls) >= 1
        assert "已停止" in final_answer_calls[0].args[2].get("text", "")
