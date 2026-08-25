import logging
import os
import time
from typing import TYPE_CHECKING, Any

from agent.core.react_agent import DEFAULT_SYSTEM_PROMPT

if TYPE_CHECKING:
    from agent.grpc_client import FlowPartnerClient
    from agent.tools.registry import ToolRegistry

# 缓存 TTL（秒）：仅作网络分区/通知丢失时的安全兜底，失效以 Go 推送的 agents_changed 为主
AGENT_CACHE_TTL_SECONDS = 30

DEFAULT_MAX_AGENT_DEPTH = 5


def max_agent_depth() -> int:
    """读取最大嵌套层级配置（FP_MAX_AGENT_DEPTH，默认 5），非法值回退默认。"""
    raw = os.environ.get("FP_MAX_AGENT_DEPTH", "")
    try:
        value = int(raw)
    except ValueError:
        return DEFAULT_MAX_AGENT_DEPTH
    if value < 1:
        return DEFAULT_MAX_AGENT_DEPTH
    return value


class AgentRegistry:


    def __init__(self, client: "FlowPartnerClient") -> None:
        self._client = client
        self._defs: dict[str, dict[str, Any]] = {}
        self._loaded_at = 0.0
        self._ttl = AGENT_CACHE_TTL_SECONDS

    async def load(self, force: bool = False) -> None:
        """按需（或强制）从 gRPC 拉取全部智能体定义并重建缓存。"""
        if not force and self._defs and (time.monotonic() - self._loaded_at) < self._ttl:
            return
        try:
            agents = await self._client.list_agents()
        except Exception as e:
            logging.error(f"[AgentRegistry] 读取智能体定义失败，降级为空列表: {e}")
            agents = []
        self._defs = {a.get("id", ""): a for a in agents if a.get("id")}
        self._loaded_at = time.monotonic()
        logging.info(f"[AgentRegistry] 智能体定义已加载: {len(self._defs)} 个")

    def invalidate(self) -> None:
        """收到 Go 的 agents_changed 通知后立即失效缓存，下次访问时重拉。"""
        self._defs = {}
        self._loaded_at = 0.0
        logging.info("[AgentRegistry] 收到 agents_changed，缓存已失效")

    async def get(self, agent_id: str) -> dict[str, Any] | None:
        await self.load()
        return self._defs.get(agent_id)

    async def list(self) -> list[dict[str, Any]]:
        await self.load()
        return list(self._defs.values())

    async def system_prompt_for(self, agent_id: str) -> str:
        """返回指定智能体的系统提示词；定义缺失/加载失败时回退默认提示词。"""
        await self.load()
        defn = self._defs.get(agent_id)
        if defn is None:
            logging.warning(
                f"[AgentRegistry] 智能体 {agent_id} 定义不可用（缺失或 gRPC 读取失败），"
                "使用默认系统提示词"
            )
            return DEFAULT_SYSTEM_PROMPT
        prompt = defn.get("system_prompt")
        if not prompt:
            logging.warning(f"[AgentRegistry] 智能体 {agent_id} 系统提示词为空，使用默认提示词")
            return DEFAULT_SYSTEM_PROMPT
        return prompt

    async def build_tool_registry(
        self,
        agent_id: str,
        session_id: str,
        depth: int,
        trace_id: str,
        parent_span_id: str,
        send_event_func=None,
    ) -> "ToolRegistry":

        from agent.tools.agent_tool import make_agent_tool_handler

        await self.load()
        registry = self._client.tool_registry.copy()
        for other_id, defn in self._defs.items():
            if other_id == agent_id:
                continue
            registry.register(
                name=f"agent__{other_id}",
                description=defn.get("description") or f"调用子智能体 {defn.get('name', other_id)} 完成指定任务",
                parameters={
                    "type": "object",
                    "properties": {
                        "task": {
                            "type": "string",
                            "description": "要交给子智能体完成的任务描述或上下文",
                        }
                    },
                    "required": ["task"],
                },
                handler=make_agent_tool_handler(
                    self._client, self, other_id, session_id, depth + 1,
                    trace_id, parent_span_id, send_event_func,
                ),
            )
        return registry
