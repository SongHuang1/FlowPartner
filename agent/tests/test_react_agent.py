import json
from unittest.mock import AsyncMock

import pytest

from agent.core.react_agent import ReactAgent
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


class TestReactAgent:
    async def _run_simple_chat(self, llm_response):
        call_llm = AsyncMock(return_value=llm_response)
        agent, send_event = make_agent(call_llm)
        result = await agent.run("Hello")
        return result, send_event, call_llm

    @pytest.mark.asyncio
    async def test_simple_response(self):
        result, send_event, call_llm = await self._run_simple_chat({
            "success": True,
            "content": "Hi there!",
            "finish_reason": "stop",
        })
        assert result == "Hi there!"
        send_event.assert_any_call("test-session", "status_update", {"status": "thinking", "iteration": 1})
        send_event.assert_any_call("test-session", "final_answer", {"text": "Hi there!"})

    @pytest.mark.asyncio
    async def test_llm_error(self):
        call_llm = AsyncMock(return_value={
            "success": False,
            "error_message": "API key invalid",
            "error_guess": "Check your API key",
        })
        agent, send_event = make_agent(call_llm)
        result = await agent.run("Hello")

        assert result == "API key invalid"
        send_event.assert_any_call("test-session", "error", {
            "message": "API key invalid",
            "guess": "Check your API key",
        })

    @pytest.mark.asyncio
    async def test_tool_call_then_final(self):
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
        result = await agent.run("Use echo tool")

        assert result == "Done!"
        send_event.assert_any_call("test-session", "tool_call", {"tool": "echo", "args": {"text": "hello"}})
        send_event.assert_any_call("test-session", "tool_result", {"tool": "echo", "result": "echo: hello"})

    @pytest.mark.asyncio
    async def test_max_iterations_exceeded(self):
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "",
            "finish_reason": "length",
        })
        agent, send_event = make_agent(call_llm)
        result = await agent.run("Loop forever")

        assert "未能得出结论" in result
        assert call_llm.call_count == agent.max_iterations

    @pytest.mark.asyncio
    async def test_incomplete_response(self):
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "",
            "finish_reason": "",
        })
        agent, send_event = make_agent(call_llm)
        result = await agent.run("Hello")

        assert result == ""
        send_event.assert_any_call("test-session", "final_answer", {"text": "", "incomplete": True})

    @pytest.mark.asyncio
    async def test_history_passed_through(self):
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
        result = await agent.run("Use tool")

        assert result == "recovered"
        send_event.assert_any_call("test-session", "tool_call", {"tool": "echo", "args": {}})

    @pytest.mark.asyncio
    async def test_tool_call_missing_id_recovers(self):
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
        result = await agent.run("Use tool")

        assert result == "done"

    @pytest.mark.asyncio
    async def test_multiple_tool_calls_in_single_response(self):
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
        result = await agent.run("Use tools")

        assert result == "done"
        send_event.assert_any_call("test-session", "tool_call", {"tool": "echo", "args": {"text": "a"}})
        send_event.assert_any_call("test-session", "tool_call", {"tool": "echo", "args": {"text": "b"}})

    @pytest.mark.asyncio
    async def test_unknown_tool_result_fed_back(self):
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
        result = await agent.run("Use tool")

        assert result == "ok"
        send_event.assert_any_call(
            "test-session", "tool_result", {"tool": "nonexistent_tool", "result": "错误：未知工具 'nonexistent_tool'"}
        )

    @pytest.mark.asyncio
    async def test_tool_result_truncated_to_500_chars(self):
        async def long_handler():
            return "x" * 800

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
        await agent.run("Use tool")

        send_event.assert_any_call("test-session", "tool_result", {"tool": "long", "result": "x" * 500})

    @pytest.mark.asyncio
    async def test_send_event_func_forwarded_to_call_llm(self):
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
    async def test_status_update_for_each_iteration(self):
        call_llm = AsyncMock(return_value={
            "success": True,
            "content": "",
            "finish_reason": "length",
        })
        agent, send_event = make_agent(call_llm)

        await agent.run("Loop")

        status_calls = [call.args for call in send_event.await_args_list if call.args[1] == "status_update"]
        assert len(status_calls) == agent.max_iterations
        assert status_calls[0][2] == {"status": "thinking", "iteration": 1}
        assert status_calls[-1][2] == {"status": "thinking", "iteration": agent.max_iterations}

    @pytest.mark.asyncio
    async def test_llm_error_without_guess(self):
        call_llm = AsyncMock(return_value={
            "success": False,
            "error_message": "boom",
        })
        agent, send_event = make_agent(call_llm)

        result = await agent.run("Hello")

        assert result == "boom"
        send_event.assert_any_call("test-session", "error", {"message": "boom"})
