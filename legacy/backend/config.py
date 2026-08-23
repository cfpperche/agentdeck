"""Central config for AgentDeck."""
import os
from pathlib import Path

HOME = Path(os.environ.get("AGENTDECK_HOME", str(Path.home())))
DATA_DIR = Path(os.environ.get("AGENTDECK_DATA", HOME / "agentdeck" / "data"))
WORKSPACES_DIR = DATA_DIR / "workspaces"
PORT = int(os.environ.get("AGENTDECK_PORT", "8444"))
HOST = os.environ.get("AGENTDECK_HOST", "0.0.0.0")

DATA_DIR.mkdir(parents=True, exist_ok=True)
WORKSPACES_DIR.mkdir(parents=True, exist_ok=True)
