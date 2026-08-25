import asyncio
import json
from pathlib import Path
from unittest.mock import AsyncMock

import grpc
import pytest

from agent import agent_pb2
from agent.grpc_client import FlowPartnerClient


def make_client(tmp_path: Path) -> FlowPartnerClient:
    return FlowPartnerClient(workspace_path=str(tmp_path))


def llm_response(json_response: str, *, is_error: bool = False) -> agent_pb2.LLMResponse:
    return agent_pb2.LLMResponse(is_error=is_error, json_response=json_response)


def tool_response(success: bool, result: str, error_code: str = "") -> agent_pb2.ToolResponse:
    return agent_pb2.ToolResponse(success=success, result=result, error_code=error_code)


def chunk(*, delta: dict | None = None, finish_reason: str = "", usage: dict | None = None) -> str:
    choice: dict = {"delta": delta or {}}
    if finish_reason:
        choice["finish_reason"] = finish_reason
    body: dict = {"choices": [choice]}
    if usage:
        body["usage"] = usage
    return json.dumps(body)


class FakeCallLLMStub:
    """Fake gRPC stub that yields pre-built LLMResponse messages from CallLLM."""

    def __init__(self, responses: list[agent_pb2.LLMResponse]) -> None:
        self.responses = responses

    async def CallLLM(self, request: agent_pb2.LLMRequest):  # noqa: N802 - mirrors gRPC stub
        for response in self.responses:
            yield response


class RaisingCallLLMStub:
    def __init__(self, exc: BaseException) -> None:
        self.exc = exc

    async def CallLLM(self, request: agent_pb2.LLMRequest):  # noqa: N802 - mirrors gRPC stub
        raise self.exc
        yield  # unreachable; makes this an async generator so iteration raises


class TestFlowPartnerClientInit:
    def test_history_dir_created(self, tmp_path: Path):
        client = make_client(tmp_path)
        assert client.history_dir == tmp_path / "history"
        assert (tmp_path / "history").is_dir()

    def test_default_tools_registered(self, tmp_path: Path):
        client = make_client(tmp_path)
        names = {
            tool["function"]["name"]
            for tool in client.tool_registry.get_openai_tools_definition()
        }
        assert names == {"read", "write", "bash", "edit", "trash", "purge"}

    def test_trash_and_purge_tools_defined(self, tmp_path: Path):
        client = make_client(tmp_path)
        definitions = client.tool_registry.get_openai_tools_definition()
        by_name = {tool["function"]["name"]: tool["function"] for tool in definitions}
        # trash: 安全删除（移入回收站）
        trash_props = by_name["trash"]["parameters"]["properties"]
        assert "path" in trash_props
        assert "paths" in trash_props
        # purge: 永久删除（需审批）
        purge_props = by_name["purge"]["parameters"]["properties"]
        assert "entry" in purge_props

    def test_default_grpc_address(self, tmp_path: Path):
        client = make_client(tmp_path)
        assert client.server_address == "localhost:50051"


class TestSendEvent:
    @pytest.mark.asyncio
    async def test_send_event_enqueues_agent_event(self, tmp_path: Path):
        client = make_client(tmp_path)
        queue: asyncio.Queue = asyncio.Queue()

        await client.send_event("sess-1", "tool_call", {"tool": "echo", "args": {"text": "hi"}}, queue)

        event = queue.get_nowait()
        assert event.session_id == "sess-1"
        assert event.event_type == "tool_call"
        assert json.loads(event.payload) == {"tool": "echo", "args": {"text": "hi"}}


class TestCallLLMViaGo:
    @pytest.mark.asyncio
    async def test_streams_content_across_chunks(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([  # type: ignore[assignment]
            llm_response(chunk(delta={"content": "Hel"})),
            llm_response(chunk(delta={"content": "lo "})),
            llm_response(chunk(delta={"content": "world"}, finish_reason="stop")),
        ])

        result = await client.call_llm_via_go("sess-1", "{}")

        assert result == {
            "success": True,
            "content": "Hello world",
            "finish_reason": "stop",
            "json_response": "",
        }

    @pytest.mark.asyncio
    async def test_reconstructs_tool_calls_from_deltas(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([  # type: ignore[assignment]
            llm_response(chunk(delta={"tool_calls": [
                {"index": 0, "id": "call_1", "type": "function", "function": {"name": "read_file", "arguments": ""}}
            ]})),
            llm_response(chunk(delta={"tool_calls": [
                {"index": 0, "function": {"arguments": '{"path": "'}}
            ]})),
            llm_response(chunk(delta={"tool_calls": [
                {"index": 0, "function": {"arguments": 'test.txt"}'}}
            ]}, finish_reason="tool_calls")),
        ])

        result = await client.call_llm_via_go("sess-1", "{}")

        assert result["success"] is True
        assert result["finish_reason"] == "tool_calls"
        assert result["tool_calls"] == [
            {
                "id": "call_1",
                "type": "function",
                "function": {"name": "read_file", "arguments": '{"path": "test.txt"}'},
            }
        ]

    @pytest.mark.asyncio
    async def test_multiple_tool_call_indices(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([  # type: ignore[assignment]
            llm_response(chunk(delta={"tool_calls": [
                {"index": 0, "id": "a", "function": {"name": "read_file", "arguments": "{}"}},
                {"index": 1, "id": "b", "function": {"name": "write_file", "arguments": '{"path": "x"}'}},
            ]}, finish_reason="tool_calls")),
        ])

        result = await client.call_llm_via_go("sess-1", "{}")

        assert [tc["id"] for tc in result["tool_calls"]] == ["a", "b"]

    @pytest.mark.asyncio
    async def test_usage_included_when_present(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([  # type: ignore[assignment]
            llm_response(chunk(delta={"content": "hi"}, finish_reason="stop",
                               usage={"prompt_tokens": 5, "completion_tokens": 3})),
        ])

        result = await client.call_llm_via_go("sess-1", "{}")

        assert result["usage"] == {"prompt_tokens": 5, "completion_tokens": 3}

    @pytest.mark.asyncio
    async def test_skips_chunks_without_choices(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([  # type: ignore[assignment]
            llm_response(json.dumps({"choices": []})),
            llm_response(json.dumps({"data": "not a chunk"})),
            llm_response(chunk(delta={"content": "ok"}, finish_reason="stop")),
        ])

        result = await client.call_llm_via_go("sess-1", "{}")

        assert result["content"] == "ok"

    @pytest.mark.asyncio
    async def test_error_response_returns_failure(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([  # type: ignore[assignment]
            llm_response(json.dumps({"message": "模型不存在", "guess": "检查模型名称"}), is_error=True),
        ])

        result = await client.call_llm_via_go("sess-1", "{}")

        assert result == {
            "success": False,
            "error_message": "模型不存在",
            "error_guess": "检查模型名称",
            "json_response": "",
        }

    @pytest.mark.asyncio
    async def test_aio_rpc_error_returns_failure(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = RaisingCallLLMStub(  # type: ignore[assignment]
            grpc.aio.AioRpcError(code=grpc.StatusCode.UNAVAILABLE, details="connection refused")
        )

        result = await client.call_llm_via_go("sess-1", "{}")

        assert result["success"] is False
        assert "gRPC error" in result["error_message"]
        assert "connection refused" in result["error_message"]

    @pytest.mark.asyncio
    async def test_malformed_json_chunk_returns_failure(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([llm_response("{not valid json")])  # type: ignore[assignment]

        result = await client.call_llm_via_go("sess-1", "{}")

        assert result["success"] is False
        assert "parse error" in result["error_message"].lower()

    @pytest.mark.asyncio
    async def test_send_event_func_receives_llm_chunks(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeCallLLMStub([  # type: ignore[assignment]
            llm_response(chunk(delta={"content": "Hel"})),
            llm_response(chunk(delta={"content": "lo"}, finish_reason="stop")),
        ])
        send_event = AsyncMock()

        await client.call_llm_via_go("sess-1", "{}", send_event_func=send_event)

        send_event.assert_any_await(
            "sess-1", "llm_chunk", {"content": "Hel", "iteration": 0}
        )
        send_event.assert_any_await(
            "sess-1", "llm_chunk", {"content": "lo", "iteration": 0}
        )


class TestHandleChat:
    @staticmethod
    def read_history_lines(tmp_path: Path, session_id: str) -> list[dict]:
        """读取 JSONL 历史文件并过滤 meta 行（无 role 字段）。"""
        history_file = tmp_path / "history" / f"{session_id}.json"
        assert history_file.is_file()
        return [
            json.loads(line)
            for line in history_file.read_text(encoding="utf-8").strip().splitlines()
            if line.strip() and json.loads(line).get("role")
        ]

    @staticmethod
    def read_history_meta(tmp_path: Path, session_id: str) -> list[dict]:
        history_file = tmp_path / "history" / f"{session_id}.json"
        return [
            json.loads(line)
            for line in history_file.read_text(encoding="utf-8").strip().splitlines()
            if line.strip() and json.loads(line).get("meta")
        ]

    @pytest.mark.asyncio
    async def test_writes_history_file(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.call_llm_via_go = AsyncMock(return_value={  # type: ignore[method-assign]
            "success": True,
            "content": "final answer",
            "finish_reason": "stop",
        })
        command = agent_pb2.ServerCommand(
            session_id="sess-1",
            command_type="start_chat",
            payload=json.dumps({"user_message": "你好"}),
        )
        queue: asyncio.Queue = asyncio.Queue()

        await client.handle_chat(command, queue)

        entries = self.read_history_lines(tmp_path, "sess-1")
        assert entries == [
            {"role": "user", "content": "你好"},
            {"role": "assistant", "content": "final answer"},
        ]

    @pytest.mark.asyncio
    async def test_writes_executor_meta_line(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.call_llm_via_go = AsyncMock(return_value={  # type: ignore[method-assign]
            "success": True,
            "content": "final answer",
            "finish_reason": "stop",
        })
        command = agent_pb2.ServerCommand(
            session_id="sess-meta",
            command_type="start_chat",
            payload=json.dumps({"user_message": "你好", "executor_agent_id": "sub-1"}),
        )
        queue: asyncio.Queue = asyncio.Queue()

        await client.handle_chat(command, queue)

        meta = self.read_history_meta(tmp_path, "sess-meta")
        assert meta == [{"meta": {"executor_agent_id": "sub-1"}}]

    @pytest.mark.asyncio
    async def test_handles_empty_payload(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.call_llm_via_go = AsyncMock(return_value={  # type: ignore[method-assign]
            "success": True,
            "content": "",
            "finish_reason": "stop",
        })
        command = agent_pb2.ServerCommand(
            session_id="sess-2",
            command_type="start_chat",
            payload="",
        )
        queue: asyncio.Queue = asyncio.Queue()

        await client.handle_chat(command, queue)

        entries = self.read_history_lines(tmp_path, "sess-2")
        assert entries[0] == {"role": "user", "content": ""}

    @pytest.mark.asyncio
    async def test_history_from_payload_passed_to_agent(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.call_llm_via_go = AsyncMock(return_value={  # type: ignore[method-assign]
            "success": True,
            "content": "final answer",
            "finish_reason": "stop",
        })
        command = agent_pb2.ServerCommand(
            session_id="sess-3",
            command_type="start_chat",
            payload=json.dumps({
                "user_message": "第二个问题",
                "history": [
                    {"role": "user", "content": "第一个问题"},
                    {"role": "assistant", "content": "第一个回答"},
                ],
            }),
        )
        queue: asyncio.Queue = asyncio.Queue()

        await client.handle_chat(command, queue)

        # 验证 history 被传给 ReactAgent.run（通过 call_llm 收到的 payload 检查）
        call_args = client.call_llm_via_go.await_args
        assert call_args is not None
        llm_payload = json.loads(call_args.args[1])
        messages = llm_payload["messages"]
        assert {"role": "user", "content": "第一个问题"} in messages
        assert {"role": "assistant", "content": "第一个回答"} in messages
        assert {"role": "user", "content": "第二个问题"} in messages

        # 历史文件应追加同一会话
        entries = self.read_history_lines(tmp_path, "sess-3")
        assert entries == [
            {"role": "user", "content": "第二个问题"},
            {"role": "assistant", "content": "final answer"},
        ]

    @pytest.mark.asyncio
    async def test_invalid_session_id_rejected(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.call_llm_via_go = AsyncMock(return_value={  # type: ignore[method-assign]
            "success": True,
            "content": "should not run",
            "finish_reason": "stop",
        })
        command = agent_pb2.ServerCommand(
            session_id="../../etc/passwd",
            command_type="start_chat",
            payload=json.dumps({"user_message": "hi"}),
        )
        queue: asyncio.Queue = asyncio.Queue()

        await client.handle_chat(command, queue)

        client.call_llm_via_go.assert_not_awaited()
        # 不应在 history 目录中创建任何文件
        assert list((tmp_path / "history").iterdir()) == []


class FakeExecuteToolStub:
    """Fake gRPC stub that returns pre-built ToolResponse from ExecuteTool."""

    def __init__(self, response: agent_pb2.ToolResponse) -> None:
        self.response = response

    async def ExecuteTool(self, request: agent_pb2.ToolRequest):  # noqa: N802
        return self.response


class RaisingExecuteToolStub:
    def __init__(self, exc: BaseException) -> None:
        self.exc = exc

    async def ExecuteTool(self, request: agent_pb2.ToolRequest):  # noqa: N802
        raise self.exc


class TestExecuteTool:
    @pytest.mark.asyncio
    async def test_success(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeExecuteToolStub(tool_response(True, "hello world"))  # type: ignore[assignment]

        result = await client.execute_tool("sess-1", "read", {"path": "test.txt"})

        assert result == {"success": True, "result": "hello world", "error_code": ""}

    @pytest.mark.asyncio
    async def test_failure(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = FakeExecuteToolStub(  # type: ignore[assignment]
            tool_response(False, "文件不存在: test.txt", "TOOL_ERROR")
        )

        result = await client.execute_tool("sess-1", "read", {"path": "test.txt"})

        assert result == {
            "success": False,
            "result": "文件不存在: test.txt",
            "error_code": "TOOL_ERROR",
        }

    @pytest.mark.asyncio
    async def test_rpc_error(self, tmp_path: Path):
        client = make_client(tmp_path)
        client.stub = RaisingExecuteToolStub(  # type: ignore[assignment]
            grpc.aio.AioRpcError(code=grpc.StatusCode.UNAVAILABLE, details="connection refused")
        )

        result = await client.execute_tool("sess-1", "read", {"path": "test.txt"})

        assert result["success"] is False
        assert "gRPC 通信失败" in result["result"]
        assert result["error_code"] == "TOOL_ERROR"

    @pytest.mark.asyncio
    async def test_stub_not_connected(self, tmp_path: Path):
        client = make_client(tmp_path)
        # stub is None by default

        result = await client.execute_tool("sess-1", "read", {"path": "test.txt"})

        assert result["success"] is False
        assert "gRPC 连接未建立" in result["result"]
        assert result["error_code"] == "TOOL_ERROR"


# --- 权限审批流程测试 ---

class FakePermissionStub:
    """Fake gRPC stub that returns needs_permission first, then success on retry."""

    def __init__(self, *, needs_perm_response: agent_pb2.ToolResponse, success_response: agent_pb2.ToolResponse):
        self.needs_perm_response = needs_perm_response
        self.success_response = success_response
        self.call_count = 0

    async def ExecuteTool(self, request: agent_pb2.ToolRequest):  # noqa: N802
        self.call_count += 1
        if request.approval_id:
            return self.success_response
        return self.needs_perm_response


class FakeDenyStub:
    """Fake stub that returns needs_permission, then failure when retried with approval_id."""

    def __init__(self, *, needs_perm_response: agent_pb2.ToolResponse, deny_response: agent_pb2.ToolResponse):
        self.needs_perm_response = needs_perm_response
        self.deny_response = deny_response
        self.call_count = 0

    async def ExecuteTool(self, request: agent_pb2.ToolRequest):  # noqa: N802
        self.call_count += 1
        if request.approval_id:
            return self.deny_response
        return self.needs_perm_response


class TestExecuteToolPermissionFlow:
    @pytest.mark.asyncio
    async def test_suspend_and_resume_with_allow(self, tmp_path: Path):
        """ExecuteTool 返回 needs_permission=True → 前端发 permission_response allow → 重试成功"""
        client = make_client(tmp_path)
        perm_resp = agent_pb2.ToolResponse(
            needs_permission=True, request_id="req-123",
            success=False, result=""
        )
        success_resp = agent_pb2.ToolResponse(
            success=True, result="file content here"
        )
        client.stub = FakePermissionStub(  # type: ignore[assignment]
            needs_perm_response=perm_resp, success_response=success_resp
        )
        # 注册活跃队列（send_event 需要）
        queue: asyncio.Queue = asyncio.Queue()
        client._active_queues["sess-perm"] = queue

        # 启动 execute_tool（会挂起等待 approval）
        async def run_tool():
            return await client.execute_tool("sess-perm", "read", {"path": "/tmp/secret.txt"})

        task = asyncio.create_task(run_tool())

        # 等待 permission_request 事件被发送到 queue
        event = await asyncio.wait_for(queue.get(), timeout=2.0)
        assert event.event_type == "permission_request"
        payload = json.loads(event.payload)
        assert payload["request_id"] == "req-123"
        assert payload["tool"] == "read"

        # 模拟前端发回 permission_response allow
        async with client._approval_lock:
            future = client._pending_approvals.get("req-123")
        assert future is not None
        future.set_result("allow")

        # 等待 execute_tool 完成
        result = await asyncio.wait_for(task, timeout=2.0)
        assert result == {"success": True, "result": "file content here", "error_code": ""}
        assert client.stub.call_count == 2  # type: ignore[union-attr]

    @pytest.mark.asyncio
    async def test_suspend_and_deny_by_user(self, tmp_path: Path):
        """ExecuteTool 返回 needs_permission=True → 用户拒绝 → 返回 PERMISSION_DENIED"""
        client = make_client(tmp_path)
        perm_resp = agent_pb2.ToolResponse(
            needs_permission=True, request_id="req-456",
            success=False, result=""
        )
        client.stub = FakeDenyStub(  # type: ignore[assignment]
            needs_perm_response=perm_resp,
            deny_response=agent_pb2.ToolResponse(success=False, result="审批无效", error_code="APPROVAL_INVALID")
        )
        queue: asyncio.Queue = asyncio.Queue()
        client._active_queues["sess-deny"] = queue

        async def run_tool():
            return await client.execute_tool("sess-deny", "write", {"path": "/tmp/secret.txt", "content": "data"})

        task = asyncio.create_task(run_tool())

        # 等待 permission_request 事件
        await asyncio.wait_for(queue.get(), timeout=2.0)

        # 模拟用户拒绝
        async with client._approval_lock:
            future = client._pending_approvals.get("req-456")
        assert future is not None
        future.set_result("deny")

        result = await asyncio.wait_for(task, timeout=2.0)
        assert result["success"] is False
        assert result["error_code"] == "PERMISSION_DENIED"
        assert "拒绝" in result["result"]

    @pytest.mark.asyncio
    async def test_drain_pending_approvals_on_disconnect(self, tmp_path: Path):
        """断连时 _drain_pending_approvals 应将所有悬挂 Future 以 deny 完成"""
        client = make_client(tmp_path)
        # 手动创建悬挂 Future
        future1: asyncio.Future = asyncio.get_event_loop().create_future()
        future2: asyncio.Future = asyncio.get_event_loop().create_future()
        client._pending_approvals["req-drain-1"] = future1
        client._pending_approvals["req-drain-2"] = future2

        client._drain_pending_approvals()

        assert future1.done()
        assert future1.result() == "deny"
        assert future2.done()
        assert future2.result() == "deny"
        assert len(client._pending_approvals) == 0


# --- cancel_task 测试 ---

class TestHandleCommandCancelTask:
    @pytest.mark.asyncio
    async def test_cancel_task_cancels_active_task(self, tmp_path: Path):
        """_cancel_task 应取消对应 session 的活跃任务"""
        client = make_client(tmp_path)
        cancel_called = asyncio.Event()

        async def slow_task():
            try:
                await asyncio.sleep(100)
            except asyncio.CancelledError:
                cancel_called.set()
                raise

        # 注册一个模拟任务
        task = asyncio.create_task(slow_task())
        client._tasks["sess-cancel"] = task

        # 让出事件循环，使 slow_task 进入 await asyncio.sleep(100)
        await asyncio.sleep(0)

        await client._cancel_task("sess-cancel")

        # 等待任务被取消
        await asyncio.wait_for(cancel_called.wait(), timeout=2.0)
        assert task.cancelled()

    @pytest.mark.asyncio
    async def test_cancel_task_no_active_task(self, tmp_path: Path):
        """_cancel_task 无活跃任务时应静默忽略"""
        client = make_client(tmp_path)
        await client._cancel_task("sess-no-task")
        assert "sess-no-task" not in client._tasks


class TestSubAgentRunTracker:
    def test_pairs_call_ids_with_spans_in_order(self):
        from agent.grpc_client import SubAgentRunTracker

        tracker = SubAgentRunTracker()
        tracker.observe("tool_call", {"tool": "agent__sub1", "call_id": "c1"})
        tracker.observe("llm_chunk", {"content": "ignored"})
        tracker.observe("tool_call", {"tool": "read", "call_id": "c-skip"})  # 非 agent 工具不配对
        tracker.observe("subagent_start", {"span_id": "s1", "agent_name": "A", "task": "t1"})
        tracker.observe("subagent_end", {"span_id": "s1", "result": "done-1"})

        assert tracker.refs == [{"call_id": "c1", "span_id": "s1"}]
        assert tracker.runs["s1"]["status"] == "done"
        assert tracker.runs["s1"]["result"] == "done-1"

    def test_collects_steps_and_error_status(self):
        from agent.grpc_client import SubAgentRunTracker

        tracker = SubAgentRunTracker()
        tracker.observe("subagent_start", {"span_id": "s2", "agent_name": "B"})
        tracker.observe("subagent_step", {"span_id": "s2", "step_type": "thinking", "content": "想"})
        tracker.observe(
            "subagent_step",
            {"span_id": "s2", "step_type": "tool_call", "tool": "read", "args": {"path": "a"}, "truncated": False},
        )
        tracker.observe("subagent_error", {"span_id": "s2", "message": "boom"})

        run = tracker.runs["s2"]
        assert run["status"] == "error"
        assert run["error"] == "boom"
        assert [s["step_type"] for s in run["steps"]] == ["thinking", "tool_call"]
        assert run["steps"][0] == {"step_type": "thinking", "content": "想"}
        assert run["steps"][1]["args"] == {"path": "a"}

    def test_attach_refs_targets_own_assistant_message(self):
        from agent.grpc_client import SubAgentRunTracker

        tracker = SubAgentRunTracker()
        tracker.observe("tool_call", {"tool": "agent__sub1", "call_id": "c1"})
        tracker.observe("subagent_start", {"span_id": "s1"})

        messages = [
            {"role": "user", "content": "hi"},
            {"role": "assistant", "content": "", "tool_calls": [{"id": "c1"}]},
            {"role": "tool", "tool_call_id": "c1", "content": "ok"},
            {"role": "assistant", "content": "final"},
        ]
        tracker.attach_refs(messages)

        assert messages[1]["subagent_refs"] == [{"call_id": "c1", "span_id": "s1"}]
        assert "subagent_refs" not in messages[3]

    def test_merge_subagents_file_accumulates_across_rounds(self, tmp_path: Path):
        import asyncio

        client = make_client(tmp_path)
        asyncio.run(client._merge_subagents_file("sess-x", {"s1": {"span_id": "s1", "status": "done"}}))
        asyncio.run(client._merge_subagents_file("sess-x", {"s2": {"span_id": "s2", "status": "error"}}))

        data = json.loads(
            (tmp_path / "history" / "sess-x.subagents.json").read_text(encoding="utf-8")
        )
        assert set(data["runs"].keys()) == {"s1", "s2"}


class TestPermissionResponseHandling:
    @pytest.mark.asyncio
    async def test_resolves_pending_future(self, tmp_path: Path):
        client = make_client(tmp_path)
        loop = asyncio.get_running_loop()
        future: asyncio.Future = loop.create_future()
        async with client._approval_lock:
            client._pending_approvals["req-1"] = future

        await client._handle_permission_response(json.dumps({"request_id": "req-1", "decision": "allow"}))

        assert future.result() == "allow"
        assert "req-1" not in client._pending_approvals

    @pytest.mark.asyncio
    async def test_unknown_request_id_is_ignored(self, tmp_path: Path):
        client = make_client(tmp_path)

        await client._handle_permission_response(json.dumps({"request_id": "missing", "decision": "deny"}))

        assert client._pending_approvals == {}

    @pytest.mark.asyncio
    async def test_malformed_payload_does_not_raise(self, tmp_path: Path):
        client = make_client(tmp_path)

        await client._handle_permission_response("{not json")

        assert client._pending_approvals == {}
