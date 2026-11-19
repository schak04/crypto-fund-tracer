"""The known exchange / VASP registry.

An attribution answers: "does this destination address belong to a known
exchange?" It is a lookup table, loaded once at startup:

    address (lowercase)  ->  VaspEntry(name, type, attribution_status, label)

IMPORTANT concept (also in docs/problem.md): the blockchain never tells you who
owns an address. "Binance owns 0x28c6..." is knowledge that comes from OUTSIDE
the chain (labels published by explorers, exchanges, analytics firms). So the
registry is a data file you curate, not something the code can discover.

We validate the file when loading it, because a typo in this file would
silently produce wrong evidence. Validation rules:
  * every address is a well-formed 0x + 40 hex address
  * no address is listed under two different entities (ambiguous attribution)
  * attribution_status is one of known / probable / unverified
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Optional

from app.domain import VaspEntry
from app.formatting import is_valid_address, normalize_address

ALLOWED_STATUSES = ("known", "probable", "unverified")
# Higher = more trustworthy. Used to choose ONE attribution when several are hit.
STATUS_RANK = {"known": 3, "probable": 2, "unverified": 1}


class VaspRegistry:
    def __init__(self, entries: list[VaspEntry]) -> None:
        self._by_address: dict[str, VaspEntry] = {}
        for entry in entries:
            existing = self._by_address.get(entry.address)
            if existing is not None:
                raise ValueError(
                    f"Address {entry.address} appears twice in the VASP registry "
                    f"({existing.name!r} and {entry.name!r})"
                )
            self._by_address[entry.address] = entry

    def __len__(self) -> int:
        return len(self._by_address)

    def lookup(self, address: str) -> Optional[VaspEntry]:
        """Return the VaspEntry for an address (any letter case), or None."""
        return self._by_address.get(normalize_address(address))

    @classmethod
    def from_file(cls, path: Path) -> "VaspRegistry":
        with open(path, encoding="utf-8") as handle:
            raw = json.load(handle)
        return cls.from_dict(raw)

    @classmethod
    def from_dict(cls, raw: dict) -> "VaspRegistry":
        entries: list[VaspEntry] = []
        for entity in raw.get("entities", []):
            name = str(entity.get("name", "")).strip()
            vasp_type = str(entity.get("type", "")).strip()
            status = str(entity.get("attribution_status", "")).strip()
            if not name or not vasp_type:
                raise ValueError(f"VASP entity needs a name and a type: {entity!r}")
            if status not in ALLOWED_STATUSES:
                raise ValueError(
                    f"VASP entity {name!r}: attribution_status must be one of {ALLOWED_STATUSES}, got {status!r}"
                )
            for item in entity.get("addresses", []):
                address = normalize_address(str(item.get("address", "")))
                if not is_valid_address(address):
                    raise ValueError(f"VASP entity {name!r}: invalid address {item.get('address')!r}")
                entries.append(
                    VaspEntry(
                        name=name,
                        type=vasp_type,
                        attribution_status=status,
                        address=address,
                        label=str(item.get("label", "")),
                    )
                )
        return cls(entries)
