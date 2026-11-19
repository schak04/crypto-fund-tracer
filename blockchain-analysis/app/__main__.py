"""Convenience entry point:  python -m app

Reads HOST / PORT / LOG_LEVEL from the environment (see .env.example) and starts
the server. Equivalent to:  uvicorn app.main:create_app --factory --port 9090
"""

import uvicorn

from app.config import Settings, load_env_file


def main() -> None:
    load_env_file()  # optional .env next to the app/ folder
    settings = Settings.from_env()
    uvicorn.run(
        "app.main:create_app",
        factory=True,
        host=settings.host,
        port=settings.port,
        log_level=settings.log_level.lower(),
    )


if __name__ == "__main__":
    main()
