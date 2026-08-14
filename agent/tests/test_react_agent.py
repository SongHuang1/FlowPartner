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
        import json
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

        import json
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

        import json
        payload = json.loads(call_llm.call_args[0][1])
        assert payload["tools"] is None
