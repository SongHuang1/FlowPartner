from unittest.mock import AsyncMock

import pytest

from agent.core.agent_registry import DEFAULT_MAX_AGENT_DEPTH, AgentRegistry, max_agent_depth
from agent.core.react_agent import DEFAULT_SYSTEM_PROMPT
from agent.tools.registry import ToolRegistry


class FakeAgentClient:
    """最小化 FlowPartnerClient 替身：仅实现 registry 需要的接口。"""

    def __init__(self, defs: list[dict], fail_list: bool = False) -> None:
        self._defs = defs
        self.fail_list = fail_list
        self.list_calls = 0
        self.tool_registry = ToolRegistry()
        # 与真实 FlowPartnerClient 一致的默认基础工具集
        for name in ("read", "write", "bash", "edit", "trash", "purge"):
            self.tool_registry.register(name, name, {"type": "object"}, lambda: "")

    async def list_agents(self) -> list[dict]:
        self.list_calls += 1
        if self.fail_list:
            raise ConnectionError("gRPC 未连接")
        return self._defs


SAMPLE_DEFS = [
    {
        "id": "agent-a",
        "name": "翻译官",
        "description": "负责中英互译",
        "system_prompt": "你是翻译专家。",
        "created_at": 1,
        "updated_at": 1,
    },
    {
        "id": "agent-b",
        "name": "代码评审员",
        "description": "负责代码评审",
        "system_prompt": "你是代码评审专家。",
        "created_at": 2,
        "updated_at": 2,
    },
]


class TestMaxAgentDepth:
    def test_default(self):
        assert max_agent_depth() == DEFAULT_MAX_AGENT_DEPTH

    def test_env_override(self, monkeypatch):
        monkeypatch.setenv("FP_MAX_AGENT_DEPTH", "3")
        assert max_agent_depth() == 3

    def test_invalid_env_falls_back(self, monkeypatch):
        monkeypatch.setenv("FP_MAX_AGENT_DEPTH", "abc")
        assert max_agent_depth() == DEFAULT_MAX_AGENT_DEPTH

    def test_zero_env_falls_back(self, monkeypatch):
        monkeypatch.setenv("FP_MAX_AGENT_DEPTH", "0")
        assert max_agent_depth() == DEFAULT_MAX_AGENT_DEPTH


class TestAgentRegistryLoad:
    @pytest.mark.asyncio
    async def test_load_gets_all_defs(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        await registry.load()
        assert client.list_calls == 1
        assert set(registry._defs) == {"agent-a", "agent-b"}

    @pytest.mark.asyncio
    async def test_cache_avoids_reload_within_ttl(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        await registry.get("agent-a")
        await registry.get("agent-b")
        assert client.list_calls == 1

    @pytest.mark.asyncio
    async def test_ttl_expiry_triggers_reload(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        registry._ttl = -1  # 强制过期
        await registry.get("agent-a")
        await registry.get("agent-a")
        assert client.list_calls == 2

    @pytest.mark.asyncio
    async def test_invalidate_clears_cache(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        await registry.get("agent-a")
        registry.invalidate()
        assert registry._defs == {}
        await registry.get("agent-a")
        assert client.list_calls == 2

    @pytest.mark.asyncio
    async def test_grpc_failure_degrades_to_empty(self):

        client = FakeAgentClient(SAMPLE_DEFS, fail_list=True)
        registry = AgentRegistry(client)
        result = await registry.get("agent-a")
        assert result is None
        assert registry._defs == {}
        assert await registry.list() == []

    @pytest.mark.asyncio
    async def test_force_reload(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        await registry.get("agent-a")
        await registry.load(force=True)
        assert client.list_calls == 2

    @pytest.mark.asyncio
    async def test_list_returns_definitions(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        items = await registry.list()
        assert [i["id"] for i in items] == ["agent-a", "agent-b"]


class TestSystemPromptFor:
    @pytest.mark.asyncio
    async def test_returns_def_prompt(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        assert await registry.system_prompt_for("agent-a") == "你是翻译专家。"

    @pytest.mark.asyncio
    async def test_missing_def_falls_back_to_default(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        assert await registry.system_prompt_for("ghost") == DEFAULT_SYSTEM_PROMPT

    @pytest.mark.asyncio
    async def test_empty_prompt_falls_back_to_default(self):
        client = FakeAgentClient([dict(SAMPLE_DEFS[0], system_prompt="")])
        registry = AgentRegistry(client)
        assert await registry.system_prompt_for("agent-a") == DEFAULT_SYSTEM_PROMPT

    @pytest.mark.asyncio
    async def test_grpc_failure_falls_back_to_default(self):
        client = FakeAgentClient(SAMPLE_DEFS, fail_list=True)
        registry = AgentRegistry(client)
        assert await registry.system_prompt_for("main") == DEFAULT_SYSTEM_PROMPT


class TestBuildToolRegistry:
    @pytest.mark.asyncio
    async def test_registers_agent_tools_excluding_self(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        tools = await registry.build_tool_registry(
            "agent-a", "sess-1", 1, "trace-1", "", None
        )
        names = {t["function"]["name"] for t in tools.get_openai_tools_definition()}
        # 自身 agent-a 不出现在自身工具集（规约 2.4）
        assert "agent__agent-a" not in names
        assert "agent__agent-b" in names

    @pytest.mark.asyncio
    async def test_tool_description_from_def(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        tools = await registry.build_tool_registry(
            "agent-a", "sess-1", 1, "trace-1", "", None
        )
        by_name = {t["function"]["name"]: t["function"] for t in tools.get_openai_tools_definition()}
        assert by_name["agent__agent-b"]["description"] == "负责代码评审"

    @pytest.mark.asyncio
    async def test_tool_requires_task_param(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        tools = await registry.build_tool_registry(
            "agent-a", "sess-1", 1, "trace-1", "", None
        )
        by_name = {t["function"]["name"]: t["function"] for t in tools.get_openai_tools_definition()}
        params = by_name["agent__agent-b"]["parameters"]
        assert params["type"] == "object"
        assert params["required"] == ["task"]
        assert "task" in params["properties"]

    @pytest.mark.asyncio
    async def test_base_tools_copied(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        tools = await registry.build_tool_registry(
            "agent-a", "sess-1", 1, "trace-1", "", None
        )
        names = {t["function"]["name"] for t in tools.get_openai_tools_definition()}
        assert {"read", "write", "bash", "edit", "trash", "purge"} <= names

    @pytest.mark.asyncio
    async def test_agent_tool_handler_executes_sub_agent(self):
        client = FakeAgentClient(SAMPLE_DEFS)
        client.call_llm_via_go = AsyncMock(return_value={
            "success": True,
            "content": "子智能体完成",
            "finish_reason": "stop",
        })
        registry = AgentRegistry(client)
        events: list[tuple[str, str, dict]] = []

        async def send_event(sid: str, etype: str, payload: dict) -> None:
            events.append((sid, etype, payload))

        tools = await registry.build_tool_registry(
            "agent-a", "sess-1", 1, "trace-root", "", send_event
        )
        result = await tools.execute("agent__agent-b", {"task": "请评审这段代码"})
        assert result == "子智能体完成"

        # 子 agent 事件序列：start → step(final_answer) → end
        types = [e[1] for e in events]
        assert types[0] == "subagent_start"
        assert types[-1] == "subagent_end"
        assert types[-2] == "subagent_step"

        for _, _, payload in events:
            assert payload["trace_id"] == "trace-root"
            assert payload["agent_id"] == "agent-b"
            assert payload["depth"] == 2
        assert events[0][2]["span_id"] != ""


class TestAgentToolDepthLimit:
    @pytest.mark.asyncio
    async def test_depth_6_rejected(self, monkeypatch):
        monkeypatch.setenv("FP_MAX_AGENT_DEPTH", "5")
        from agent.tools.agent_tool import make_agent_tool_handler

        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        handler = make_agent_tool_handler(
            client, registry, "agent-b", "sess-1", 6, "trace-1", "", None
        )
        result = await handler(task="任务")
        assert "嵌套层级已达上限" in result
        assert "5" in result

    @pytest.mark.asyncio
    async def test_missing_agent_returns_error(self):
        from agent.tools.agent_tool import make_agent_tool_handler

        client = FakeAgentClient(SAMPLE_DEFS)
        registry = AgentRegistry(client)
        handler = make_agent_tool_handler(
            client, registry, "ghost", "sess-1", 2, "trace-1", "", None
        )
        result = await handler(task="任务")
        assert "不存在或已被删除" in result

    @pytest.mark.asyncio
    async def test_depth_within_limit_runs(self):
        from agent.tools.agent_tool import make_agent_tool_handler

        client = FakeAgentClient(SAMPLE_DEFS)
        client.call_llm_via_go = AsyncMock(return_value={
            "success": True,
            "content": "ok",
            "finish_reason": "stop",
        })
        registry = AgentRegistry(client)
        events: list[tuple[str, str, dict]] = []

        async def send_event(sid: str, etype: str, payload: dict) -> None:
            events.append((sid, etype, payload))

        handler = make_agent_tool_handler(
            client, registry, "agent-b", "sess-1", 2, "trace-1", "", send_event
        )
        result = await handler(task="任务")
        assert result == "ok"
