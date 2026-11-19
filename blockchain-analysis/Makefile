# Developer shortcuts. Usage: `make <target>` from the blockchain-analysis/ folder.
PY ?= python3

.PHONY: install test run run-live smoke-live

install:            ## install runtime + test dependencies
	$(PY) -m pip install -r requirements-dev.txt

test:               ## run the whole test suite
	$(PY) -m pytest -q

run:                ## start the service with the MOCK blockchain (no API key needed), port 9090
	PROVIDER=mock $(PY) -m app

run-live:           ## start the service with REAL Etherscan data (needs ETHERSCAN_API_KEY in env or .env)
	PROVIDER=etherscan $(PY) -m app

smoke-live:         ## one real Etherscan lookup to verify your API key + network (needs ETHERSCAN_API_KEY)
	PROVIDER=etherscan $(PY) scripts/smoke_live.py
