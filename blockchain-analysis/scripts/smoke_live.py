"""One real lookup against the configured provider, to verify your setup.

    ETHERSCAN_API_KEY=xxxx PROVIDER=etherscan python scripts/smoke_live.py [address] [depth]

It runs the full pipeline (provider -> tracer -> builder -> self-check) and prints
the JSON the Go backend would receive. Use depth 1 first: it is only a few API calls.
"""
import asyncio
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from app.config import Settings, load_env_file  # noqa: E402
from app.models import AnalysisRequest  # noqa: E402
from app.providers import build_provider  # noqa: E402
from app.service import AnalysisService  # noqa: E402
from app.vasp import VaspRegistry  # noqa: E402


async def main() -> None:
    load_env_file()
    settings = Settings.from_env()
    address = sys.argv[1] if len(sys.argv) > 1 else "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
    depth = int(sys.argv[2]) if len(sys.argv) > 2 else 1
    provider = build_provider(settings)
    try:
        service = AnalysisService(provider, VaspRegistry.from_file(settings.vasp_file), settings)
        response = await service.analyze(AnalysisRequest(wallet_address=address, max_depth=depth))
        print(response.model_dump_json(indent=2))
    finally:
        await provider.aclose()


asyncio.run(main())
