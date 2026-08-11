import logging
from pathlib import Path

async def read_file(path: str) -> str:
    """读取本地文件内容"""
    logging.info(f"[Tool] 读取文件: {path}")
    file_path = Path(path)
    if not file_path.exists():
        return f"文件不存在: {path}"
    if not file_path.is_file():
        return f"路径不是文件: {path}"
    try:
        content = file_path.read_text(encoding="utf-8")
        # 防止文件过大撑爆上下文
        if len(content) > 10000:
            content = content[:10000] + "\n... [文件过长，已截断]"
        return content
    except Exception as e:
        return f"读取失败: {str(e)}"

async def write_file(path: str, content: str) -> str:
    """写入内容到本地文件"""
    logging.info(f"[Tool] 写入文件: {path}")
    try:
        file_path = Path(path)
        file_path.parent.mkdir(parents=True, exist_ok=True)
        file_path.write_text(content, encoding="utf-8")
        return f"成功写入 {len(content)} 字符到 {path}"
    except Exception as e:
        return f"写入失败: {str(e)}"

async def list_directory(path: str) -> str:
    """列出目录内容"""
    logging.info(f"[Tool] 列出目录: {path}")
    dir_path = Path(path)
    if not dir_path.exists():
        return f"目录不存在: {path}"
    if not dir_path.is_dir():
        return f"路径不是目录: {path}"
    try:
        items = []
        for item in dir_path.iterdir():
            items.append(f"{item.name}")
        return "\n".join(items) if items else "(空目录)"
    except Exception as e:
        return f"列出目录失败: {str(e)}"