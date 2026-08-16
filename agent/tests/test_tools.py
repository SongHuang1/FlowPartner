import asyncio
from pathlib import Path

from agent.tools.file_ops import list_directory, read_file, write_file
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


class TestReadFile:
    def test_read_existing_file(self, tmp_path: Path):
        test_file = tmp_path / "test.txt"
        test_file.write_text("hello world", encoding="utf-8")

        result = asyncio.run(read_file(str(test_file)))
        assert result == "hello world"

    def test_read_nonexistent_file(self, tmp_path: Path):
        result = asyncio.run(read_file(str(tmp_path / "missing.txt")))
        assert "找不到" in result

    def test_read_directory_returns_error(self, tmp_path: Path):
        result = asyncio.run(read_file(str(tmp_path)))
        assert "不是文件" in result

    def test_read_large_file_truncated(self, tmp_path: Path):
        test_file = tmp_path / "large.txt"
        test_file.write_text("x" * 15000, encoding="utf-8")

        result = asyncio.run(read_file(str(test_file)))
        assert "截断" in result
        assert len(result) <= 10100

    def test_read_unicode_content(self, tmp_path: Path):
        test_file = tmp_path / "unicode.txt"
        test_file.write_text("中文测试 日本語 🎉", encoding="utf-8")

        result = asyncio.run(read_file(str(test_file)))
        assert result == "中文测试 日本語 🎉"


class TestWriteFile:
    def test_write_new_file(self, tmp_path: Path):
        test_file = tmp_path / "output.txt"
        result = asyncio.run(write_file(str(test_file), "hello"))

        assert "成功" in result
        assert test_file.read_text(encoding="utf-8") == "hello"

    def test_write_creates_parent_dirs(self, tmp_path: Path):
        test_file = tmp_path / "sub" / "dir" / "file.txt"
        result = asyncio.run(write_file(str(test_file), "content"))

        assert "成功" in result
        assert test_file.exists()

    def test_write_overwrites_existing(self, tmp_path: Path):
        test_file = tmp_path / "existing.txt"
        test_file.write_text("old", encoding="utf-8")

        asyncio.run(write_file(str(test_file), "new"))
        assert test_file.read_text(encoding="utf-8") == "new"

    def test_write_unicode_content(self, tmp_path: Path):
        test_file = tmp_path / "unicode.txt"
        asyncio.run(write_file(str(test_file), "中文 🎉"))
        assert test_file.read_text(encoding="utf-8") == "中文 🎉"

    def test_write_empty_content(self, tmp_path: Path):
        test_file = tmp_path / "empty.txt"
        result = asyncio.run(write_file(str(test_file), ""))
        assert "成功" in result
        assert test_file.read_text(encoding="utf-8") == ""


class TestListDirectory:
    def test_list_directory_with_files(self, tmp_path: Path):
        (tmp_path / "a.txt").write_text("a")
        (tmp_path / "b.txt").write_text("b")

        result = asyncio.run(list_directory(str(tmp_path)))
        assert "a.txt" in result
        assert "b.txt" in result

    def test_list_empty_directory(self, tmp_path: Path):
        result = asyncio.run(list_directory(str(tmp_path)))
        assert "空目录" in result

    def test_list_nonexistent_directory(self):
        result = asyncio.run(list_directory("/nonexistent/path/12345"))
        assert "找不到" in result

    def test_list_file_returns_error(self, tmp_path: Path):
        test_file = tmp_path / "file.txt"
        test_file.write_text("content")

        result = asyncio.run(list_directory(str(test_file)))
        assert "不是目录" in result

    def test_list_includes_subdirectories(self, tmp_path: Path):
        (tmp_path / "subdir").mkdir()
        (tmp_path / "file.txt").write_text("content")

        result = asyncio.run(list_directory(str(tmp_path)))
        assert "subdir" in result
        assert "file.txt" in result
