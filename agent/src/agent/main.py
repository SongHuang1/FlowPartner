import logging
from pathlib import Path

# 配置标准日志，正规项目必备
logging.basicConfig(
    level=logging.INFO, 
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

def init_workspace():
    """初始化 .flowpartner 工作目录"""
    home_dir = Path.home()
    workspace_dir = home_dir / ".flowpartner"

    if not workspace_dir.exists():
        workspace_dir.mkdir()
        logging.info(f"已创建 workspace 目录: {workspace_dir}")
    else:
        logging.info(f"workspace 目录已存在: {workspace_dir}")

    return workspace_dir

if __name__ == "__main__":
    logging.info("Agent 启动中...")
    workspace = init_workspace()
    
    logging.info(f"Agent 初始化完成，工作目录为: {workspace}")
