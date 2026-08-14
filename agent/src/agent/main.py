import logging
import asyncio
from pathlib import Path
from grpc_client import FlowPartnerClient

logging.basicConfig(
    level=logging.INFO, 
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

def init_workspace():
    home_dir = Path.home()
    workspace_dir = home_dir / ".flowpartner"
    if not workspace_dir.exists():
        workspace_dir.mkdir()
    return str(workspace_dir)

async def main():
    logging.info("Agent starting...")
    workspace = init_workspace()
    logging.info(f"Agent initialized, working directory: {workspace}")

    # 启动 gRPC 客户端 (目前 Go 还没写好 Server，所以连接会失败，这是预期的)
    client = FlowPartnerClient(workspace_path=workspace)
    await client.connect_and_listen()

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logging.info("Agent received exit signal, shutting down...")