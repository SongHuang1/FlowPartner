import logging
from pathlib import Path

async def read_file(path: str) -> str:
    """读取本地文件内容"""
    logging.info(f"[Tool] Reading file: {path}")
    file_path = Path(path)
    if not file_path.exists():
        return f"File not found: {path}"
    if not file_path.is_file():
        return f"Path is not a file: {path}"
    try:
        content = file_path.read_text(encoding="utf-8")
        # 防止文件过大撑爆上下文
        if len(content) > 10000:
            content = content[:10000] + "\n... [file too long, truncated]"
        return content
    except Exception as e:
        return f"Failed to read file: {str(e)}"

async def write_file(path: str, content: str) -> str:
    """写入内容到本地文件"""
    logging.info(f"[Tool] Writing file: {path}")
    try:
        file_path = Path(path)
        file_path.parent.mkdir(parents=True, exist_ok=True)
        file_path.write_text(content, encoding="utf-8")
        return f"Successfully wrote {len(content)} characters to {path}"
    except Exception as e:
        return f"Failed to write file: {str(e)}"

async def list_directory(path: str) -> str:
    """列出目录内容"""
    logging.info(f"[Tool] Listing directory: {path}")
    dir_path = Path(path)
    if not dir_path.exists():
        return f"Directory not found: {path}"
    if not dir_path.is_dir():
        return f"Path is not a directory: {path}"
    try:
        items = []
        for item in dir_path.iterdir():
            items.append(f"{item.name}")
        return "\n".join(items) if items else "(empty directory)"
    except Exception as e:
        return f"Failed to list directory: {str(e)}"