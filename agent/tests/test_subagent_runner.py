from unittest.mock import AsyncMock

import pytest

from agent.core.agent_registry import AgentRegistry
from agent.core.subagent_runner import SubAgentRunner
from agent.tools.registry import ToolRegistry


class FakeAgentClient:
    def __init__(self, defs: list[dict]) -> None:
        self._defs = defs
        self.tool_registry = ToolRegistry()
        for name in ("read", "write", "bash", "edit", "trash", "purge"):
            self.tool_registry.register(name, name, {"type": "object"}, lambda: "")
        self.call_llm_via_go = AsyncMock(return_value={
            "success": True,
            "content": "ok",
            "finish_reason": "stop",
        })

    async def list_agents(self) -> list[dict]:
        return self._defs


def make_runner(*, llm_result: dict | None = None, events=None):
    defs = [
        {
            "id": "agent-a",
            "name": "翻译官",
            "description": "负责中英互译",
            "system_prompt": "你是翻译专家。",
        },
        {
            "id": "agent-b",
            "name": "代码评审员",
            "description": "负责代码评审",
            "system_prompt": "你是代码评审专家。",
        },
    ]
    client = FakeAgentClient(defs)
    if llm_result is not None:
        client.call_llm_via_go = AsyncMock(return_value=llm_result)
    if events is None:
        events = []

    async def send_event(sid: str, etype: str, payload: dict) -> None:
        events.append((sid, etype, payload))

    registry = AgentRegistry(client)  # type: ignore[arg-type]
    runner = SubAgentRunner(
        client=client,  # type: ignore[arg-type]
        registry=registry,
        agent_def=defs[1],  # agent-b
        session_id="sess-root",
        depth=2,
        trace_id="trace-root",
        parent_span_id="span-root",
        task="请评审这段代码",
        send_event_func=send_event,
    )
    return runner, events


class TestSubAgentRunner:
    @pytest.mark.asyncio
    async def test_returns_final_result(self):
        runner, _ = make_runner(llm_result={
            "success": True,
            "content": "子智能体完成",
            "finish_reason": "stop",
        })
        result = await runner.run()
        assert result == "子智能体完成"

    @pytest.mark.asyncio
    async def test_emits_start_step_end_sequence(self):
        runner, events = make_runner(llm_result={
            "success": True,
            "content": "子智能体完成",
            "finish_reason": "stop",
        })
        await runner.run()

        types = [e[1] for e in events]
        assert types[0] == "subagent_start"
        assert "subagent_step" in types
        assert types[-1] == "subagent_end"

    @pytest.mark.asyncio
    async def test_events_carry_trace_span_metadata(self):
        runner, events = make_runner(llm_result={
            "success": True,
            "content": "子智能体完成",
            "finish_reason": "stop",
        })
        await runner.run()

        for _, _, payload in events:
            assert payload["trace_id"] == "trace-root"
            assert payload["parent_span_id"] == "span-root"
            assert payload["span_id"] == runner.span_id
            assert payload["agent_id"] == "agent-b"
            assert payload["agent_name"] == "代码评审员"
            assert payload["depth"] == 2

    @pytest.mark.asyncio
    async def test_final_answer_step_contains_result(self):
        runner, events = make_runner(llm_result={
            "success": True,
            "content": "结论是安全的",
            "finish_reason": "stop",
        })
        await runner.run()

        steps = [e[2] for e in events if e[1] == "subagent_step"]
        assert steps[-1]["step_type"] == "final_answer"
        assert steps[-1]["content"] == "结论是安全的"

    @pytest.mark.asyncio
    async def test_tool_call_and_result_steps(self):
        llm = AsyncMock(side_effect=[
            {
                "success": True,
                "content": "",
                "finish_reason": "tool_calls",
                "tool_calls": [
                    {
                        "id": "c1",
                        "type": "function",
                        "function": {"name": "read", "arguments": '{"path": "/tmp/a.txt"}'},
                    }
                ],
            },
            {
                "success": True,
                "content": "完成",
                "finish_reason": "stop",
            },
        ])
        runner, events = make_runner(llm_result=None)
        runner.client.call_llm_via_go = llm
        await runner.run()

        steps = [e[2] for e in events if e[1] == "subagent_step"]
        step_types = [s["step_type"] for s in steps]
        assert "tool_call" in step_types
        assert "tool_result" in step_types
        tool_call_step = next(s for s in steps if s["step_type"] == "tool_call")
        assert tool_call_step["tool"] == "read"

    @pytest.mark.asyncio
    async def test_error_emits_subagent_error(self):
        llm = AsyncMock(return_value={
            "success": False,
            "error_message": "boom",
        })
        runner, events = make_runner(llm_result=None)
        runner.client.call_llm_via_go = llm
        result = await runner.run()

        assert result == "boom"
        error_events = [e for e in events if e[1] == "subagent_error"]
        assert len(error_events) >= 1

    @pytest.mark.asyncio
    async def test_nested_sub_agent_registry_excludes_self(self):
        runner, events = make_runner(llm_result={
            "success": True,
            "content": "ok",
            "finish_reason": "stop",
        })
        await runner.registry.load()
        tools = await runner.registry.build_tool_registry(
            "agent-b", "sess-root", runner.depth, runner.trace_id, runner.span_id, runner.send_event_func
        )
        names = {t["function"]["name"] for t in tools.get_openai_tools_definition()}
        assert "agent__agent-b" not in names
        assert "agent__agent-a" in names

    @pytest.mark.asyncio
    async def test_independent_span_id_per_instance(self):
        r1, _ = make_runner(llm_result={"success": True, "content": "x", "finish_reason": "stop"})
        r2, _ = make_runner(llm_result={"success": True, "content": "y", "finish_reason": "stop"})
        assert r1.span_id != r2.span_id
        assert r1 is not r2
