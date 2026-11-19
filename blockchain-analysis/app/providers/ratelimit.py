"""A tiny client-side rate limiter.

Free explorer API plans allow only a few calls per second. If we exceed that the
provider answers "rate limit reached" and our investigation fails. It is much
cheaper to politely wait a fraction of a second than to fail, so we enforce a
minimum gap between two calls ourselves.

How it works: all callers pass through `wait()`. A lock lets one caller at a
time compute "when is the next call allowed?", sleep until then, and push the
next-allowed time forward by `min_interval`. Concurrent tasks therefore get
spread out automatically, even if they all start at the same instant.
"""

from __future__ import annotations

import asyncio
import time


class RateLimiter:
    def __init__(self, min_interval_seconds: float) -> None:
        self._min_interval = max(0.0, min_interval_seconds)
        self._lock = asyncio.Lock()
        self._next_allowed = 0.0

    async def wait(self) -> None:
        if self._min_interval == 0:
            return
        async with self._lock:
            now = time.monotonic()
            delay = self._next_allowed - now
            if delay > 0:
                await asyncio.sleep(delay)
                now = time.monotonic()
            self._next_allowed = now + self._min_interval
