import logging
import uuid
from collections.abc import Callable
from typing import TYPE_CHECKING, Any

from agent.core.react_agent import ReactAgent

if TYPE_CHECKING:
    from agent.core.agent_registry import AgentRegistry
    from agent.grpc_client import FlowPartnerClient

# 事件发送函数签名：session_id, event_type, payload
SendEventFunc = Callable[[str, str, dict], Any]


class SubAgentRunner:


    def __init__(
        self,
        client: "FlowPartnerClient",
        registry: "AgentRegistry",
        agent_def: dict[str, Any],
        session_id: str,
        depth: int,
        trace_id: str,
        parent_span_id: str,
        task: str,
        send_event_func: SendEventFunc,
    ) -> None:
        self.client = client
        self.registry = registry
        self.agent_def = agent_def
        self.agent_id = agent_def.get("id", "")
        self.agent_name = agent_def.get("name", self.agent_id)
        self.session_id = session_id
        self.depth = depth
        self.trace_id = trace_id
        self.parent_span_id = parent_span_id
        self.span_id = str(uuid.uuid4())
        self.task = task
        self.send_event_func = send_event_func

    async def _emit(self, event_type: str, payload: dict) -> None:
        """向根会话发送子 agent 事件（携带 trace/span 关联）。"""
        await self.send_event_func(
            self.session_id,
            event_type,
            {
                "agent_id": self.agent_id,
                "agent_name": self.agent_name,
                "depth": self.depth,
                "span_id": self.span_id,
                "trace_id": self.trace_id,
                "parent_span_id": self.parent_span_id,
                **payload,
            },
        )

    async def _forward(self, session_id: str, event_type: str, payload: dict) -> None:
        if event_type == "llm_chunk":
            await self._emit("subagent_step", {"step_type": "thinking", "content": payload.get("content", "")})
        elif event_type == "tool_call":
            await self._emit(
                "subagent_step",
                {
                    "step_type": "tool_call",
                    "tool": payload.get("tool", ""),
                    "args": payload.get("args", {}),
                },
            )
        elif event_type == "tool_result":
            await self._emit(
                "subagent_step",
                {
                    "step_type": "tool_result",
                    "tool": payload.get("tool", ""),
                    "result": payload.get("result", ""),
                    "truncated": payload.get("truncated", False),
                },
            )
        elif event_type == "loop_terminated":
            await self._emit(
                "subagent_step",
                {"step_type": "thinking", "content": f"循环被终止：{payload.get('reason', 'unknown')}"},
            )
        elif event_type == "error":
            await self._emit("subagent_error", {"message": payload.get("message", "未知错误")})
        elif event_type == "final_answer":
            await self._emit("subagent_step", {"step_type": "final_answer", "content": payload.get("text", "")})

    async def run(self) -> str:
        """执行子 ReAct 循环，返回最终文本结果。"""
        try:
            await self._emit(
                "subagent_start",
                {"task": self.task[:2000], "status": "running"},
            )

            tool_registry = await self.registry.build_tool_registry(
                self.agent_id, self.session_id, self.depth, self.trace_id, self.span_id, self.send_event_func
            )
            agent = ReactAgent(
                session_id=self.session_id,
                call_llm_func=self.client.call_llm_via_go,
                send_event_func=self._forward,
                tool_registry=tool_registry,
            )
            result, _ = await agent.run(
                self.task,
                system_prompt=self.agent_def.get("system_prompt"),
                send_event_func=self._forward,
            )

            await self._emit("subagent_end", {"status": "done", "result": result})
            return result
        except Exception as e:
            logging.error(f"[SubAgent] {self.agent_id} 子循环异常: {e}", exc_info=True)
            await self._emit("subagent_error", {"status": "error", "message": str(e)})
            return f"子智能体执行失败（{self.agent_name}）：{e}"
