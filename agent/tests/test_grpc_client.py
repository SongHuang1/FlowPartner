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
        assert names == {"read", "write", "bash", "edit"}

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
            "sess-1", "llm_chunk", {"content": "Hel", "finish_reason": None}
        )
        send_event.assert_any_await(
            "sess-1", "llm_chunk", {"content": "lo", "finish_reason": "stop"}
        )


class TestHandleChat:
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

        history_file = tmp_path / "history" / "sess-1.json"
        assert history_file.is_file()
        entries = json.loads(history_file.read_text(encoding="utf-8").strip())
        assert entries == [
            {"role": "user", "content": "你好"},
            {"role": "assistant", "content": "final answer"},
        ]

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

        history_file = tmp_path / "history" / "sess-2.json"
        entries = json.loads(history_file.read_text(encoding="utf-8").strip())
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
        history_file = tmp_path / "history" / "sess-3.json"
        entries = json.loads(history_file.read_text(encoding="utf-8").strip())
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
