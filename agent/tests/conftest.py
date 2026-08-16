import sys
from pathlib import Path

_SRC_DIR = Path(__file__).parent.parent / "src"
for _P in (_SRC_DIR, _SRC_DIR / "agent"):
    if str(_P) not in sys.path:
        sys.path.insert(0, str(_P))
