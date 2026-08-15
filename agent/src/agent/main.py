import logging
import asyncio
import os
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

def get_grpc_address():
    port = os.environ.get('AGENT_GRPC_PORT')
    if port:
        return f"localhost:{port}"
    return "localhost:50051"

async def main():
    logging.info("Agent starting...")
    workspace = init_workspace()
    logging.info(f"Agent initialized, working directory: {workspace}")

    grpc_address = get_grpc_address()
    logging.info(f"Connecting to Go backend at {grpc_address}")
    client = FlowPartnerClient(workspace_path=workspace, server_address=grpc_address)
    await client.connect_and_listen()

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logging.info("Agent received exit signal, shutting down...")