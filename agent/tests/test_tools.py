import asyncio
from unittest.mock import AsyncMock

from agent.tools.file_ops import (
    make_bash_handler,
    make_edit_handler,
    make_purge_handler,
    make_read_handler,
    make_trash_handler,
    make_write_handler,
)
from agent.tools.registry import ToolRegistry


class TestToolRegistry:
    def test_register_and_get_definition(self):
        registry = ToolRegistry()
        registry.register(
            name="test_tool",
            description="A test tool",
            parameters={"type": "object", "properties": {"x": {"type": "integer"}}},
            handler=lambda x: x,
        )

        defs = registry.get_openai_tools_definition()
        assert len(defs) == 1
        assert defs[0]["type"] == "function"
        assert defs[0]["function"]["name"] == "test_tool"
        assert defs[0]["function"]["description"] == "A test tool"
        assert defs[0]["function"]["parameters"]["properties"]["x"]["type"] == "integer"

    def test_multiple_tools(self):
        registry = ToolRegistry()
        registry.register("tool_a", "desc a", {}, lambda: None)
        registry.register("tool_b", "desc b", {}, lambda: None)

        defs = registry.get_openai_tools_definition()
        assert len(defs) == 2
        names = {d["function"]["name"] for d in defs}
        assert names == {"tool_a", "tool_b"}

    def test_execute_known_tool(self):
        registry = ToolRegistry()

        async def echo_handler(text):
            return f"echo: {text}"

        registry.register("echo", "Echo input", {"type": "object", "properties": {"text": {"type": "string"}}}, echo_handler)

        result = asyncio.run(registry.execute("echo", {"text": "hello"}))
        assert result == "echo: hello"

    def test_execute_unknown_tool(self):
        registry = ToolRegistry()
        result = asyncio.run(registry.execute("nonexistent", {}))
        assert "错误" in result or "unknown" in result.lower()

    def test_execute_tool_raises_exception(self):
        registry = ToolRegistry()

        async def failing_handler(**kwargs):
            raise ValueError("something went wrong")

        registry.register("failing", "fails", {}, failing_handler)
        result = asyncio.run(registry.execute("failing", {}))
        assert "错误" in result or "error" in result.lower()

    def test_empty_registry_returns_empty_list(self):
        registry = ToolRegistry()
        assert registry.get_openai_tools_definition() == []


class TestReadBridge:
    def test_read_success(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "file content here",
            "error_code": "",
        })
        handler = make_read_handler(mock_client)

        result = asyncio.run(handler(path="test.txt"))
        assert result == "file content here"
        mock_client.execute_tool.assert_called_once_with("", "read", {"path": "test.txt"})

    def test_read_failure(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": False,
            "result": "文件不存在: test.txt",
            "error_code": "TOOL_ERROR",
        })
        handler = make_read_handler(mock_client)

        result = asyncio.run(handler(path="test.txt"))
        assert "文件不存在" in result


class TestWriteBridge:
    def test_write_success(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "成功写入 5 个字符",
            "error_code": "",
        })
        handler = make_write_handler(mock_client)

        result = asyncio.run(handler(path="output.txt", content="hello"))
        assert "成功写入" in result
        mock_client.execute_tool.assert_called_once_with("", "write", {"path": "output.txt", "content": "hello"})

    def test_write_outside_workspace(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": False,
            "result": "操作被拒绝：目标路径超出工作目录范围",
            "error_code": "PATH_OUTSIDE_WORKSPACE",
        })
        handler = make_write_handler(mock_client)

        result = asyncio.run(handler(path="/etc/passwd", content="nope"))
        assert "超出工作目录范围" in result


class TestBashBridge:
    def test_bash_success(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "hello world\n",
            "error_code": "",
        })
        handler = make_bash_handler(mock_client)

        result = asyncio.run(handler(command="echo hello world"))
        assert "hello world" in result
        mock_client.execute_tool.assert_called_once_with("", "bash", {"command": "echo hello world"})

    def test_bash_failure(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": False,
            "result": "命令执行失败: exit status 1",
            "error_code": "TOOL_ERROR",
        })
        handler = make_bash_handler(mock_client)

        result = asyncio.run(handler(command="exit 1"))
        assert "命令执行失败" in result


class TestEditBridge:
    def test_edit_success(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "成功替换 1 处",
            "error_code": "",
        })
        handler = make_edit_handler(mock_client)

        result = asyncio.run(handler(path="file.txt", old_string="old", new_string="new"))
        assert result == "成功替换 1 处"
        mock_client.execute_tool.assert_called_once_with("", "edit", {
            "path": "file.txt",
            "old_string": "old",
            "new_string": "new",
        })

    def test_edit_no_match(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": False,
            "result": "未找到匹配内容",
            "error_code": "TOOL_ERROR",
        })
        handler = make_edit_handler(mock_client)

        result = asyncio.run(handler(path="file.txt", old_string="nonexistent", new_string="new"))
        assert "未找到匹配内容" in result

    def test_edit_multiple_matches(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": False,
            "result": "匹配数 3 大于 1，请提供更精确的匹配内容",
            "error_code": "EDIT_MATCH_COUNT_ERROR",
        })
        handler = make_edit_handler(mock_client)

        result = asyncio.run(handler(path="file.txt", old_string="abc", new_string="xyz"))
        assert "匹配数 3 大于 1" in result


class TestTrashBridge:
    def test_trash_single_path(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "已移入回收站: old.log",
            "error_code": "",
        })
        handler = make_trash_handler(mock_client)

        result = asyncio.run(handler(path="old.log"))
        assert "old.log" in result
        assert '"success": true' in result
        mock_client.execute_tool.assert_called_once_with("", "trash", {"path": "old.log"})

    def test_trash_multiple_paths(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "已移入回收站: a.log, b.log",
            "error_code": "",
        })
        handler = make_trash_handler(mock_client)

        result = asyncio.run(handler(paths=["a.log", "b.log"]))
        assert '"success": true' in result
        mock_client.execute_tool.assert_called_once_with("", "trash", {"paths": ["a.log", "b.log"]})

    def test_trash_blocked_result_keeps_error_code(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": False,
            "result": "删除操作被拦截，请改用 trash 工具",
            "error_code": "TOOL_DELETION_BLOCKED",
        })
        handler = make_trash_handler(mock_client)

        result = asyncio.run(handler(path="x.log"))
        assert "TOOL_DELETION_BLOCKED" in result
        assert '"success": false' in result


class TestPurgeBridge:
    def test_purge_all(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "已永久删除 2 个条目",
            "error_code": "",
        })
        handler = make_purge_handler(mock_client)

        result = asyncio.run(handler())
        assert "已永久删除" in result
        mock_client.execute_tool.assert_called_once_with("", "purge", {})

    def test_purge_single_entry(self):
        mock_client = AsyncMock()
        mock_client.execute_tool = AsyncMock(return_value={
            "success": True,
            "result": "已永久删除 1 个条目",
            "error_code": "",
        })
        handler = make_purge_handler(mock_client)

        result = asyncio.run(handler(entry="20260101T000000000001Z__1__a.log"))
        assert '"success": true' in result
        mock_client.execute_tool.assert_called_once_with("", "purge", {"entry": "20260101T000000000001Z__1__a.log"})
