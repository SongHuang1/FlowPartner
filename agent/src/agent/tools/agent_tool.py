import logging
from typing import TYPE_CHECKING

from agent.core.agent_registry import max_agent_depth

if TYPE_CHECKING:
    from agent.core.agent_registry import AgentRegistry
    from agent.grpc_client import FlowPartnerClient


def make_agent_tool_handler(
    client: "FlowPartnerClient",
    registry: "AgentRegistry",
    agent_id: str,
    session_id: str,
    depth: int,
    trace_id: str,
    parent_span_id: str,
    send_event_func=None,
):


    async def agent_tool(task: str = "") -> str:
        from agent.core.subagent_runner import SubAgentRunner

        defn = await registry.get(agent_id)
        if defn is None:
            logging.warning(f"[AgentTool] 子智能体 {agent_id} 不存在或已被删除")
            return f"子智能体调用失败：智能体不存在或已被删除（id={agent_id}）"

        limit = max_agent_depth()
        if depth > limit:
            logging.warning(
                f"[AgentTool] 拒绝调用 {agent_id}: 嵌套层级 {depth} 超过上限 {limit}"
            )
            return (
                f"子智能体调用失败：嵌套层级已达上限（{limit} 层），已拒绝调用"
                f"（{defn.get('name', agent_id)}）。请将任务拆分后直接交给当前层级的其他子智能体。"
            )

        runner = SubAgentRunner(
            client=client,
            registry=registry,
            agent_def=defn,
            session_id=session_id,
            depth=depth,
            trace_id=trace_id,
            parent_span_id=parent_span_id,
            task=task,
            send_event_func=send_event_func,
        )
        return await runner.run()

    return agent_tool
