from pathlib import Path

import agent.main as main


class TestGetGrpcAddress:
    def test_default_address(self, monkeypatch):
        monkeypatch.delenv("AGENT_GRPC_PORT", raising=False)
        assert main.get_grpc_address() == "localhost:50051"

    def test_uses_configured_port(self, monkeypatch):
        monkeypatch.setenv("AGENT_GRPC_PORT", "50052")
        assert main.get_grpc_address() == "localhost:50052"


class TestInitWorkspace:
    def test_creates_flowpartner_home(self, monkeypatch, tmp_path: Path):
        monkeypatch.setattr(main.Path, "home", lambda: tmp_path)
        result = main.init_workspace()
        assert result == str(tmp_path / ".flowpartner")
        assert (tmp_path / ".flowpartner").is_dir()

    def test_existing_workspace_not_recreated(self, monkeypatch, tmp_path: Path):
        existing = tmp_path / ".flowpartner"
        existing.mkdir()
        marker = existing / "marker.txt"
        marker.write_text("keep", encoding="utf-8")

        monkeypatch.setattr(main.Path, "home", lambda: tmp_path)
        main.init_workspace()

        assert marker.read_text(encoding="utf-8") == "keep"
