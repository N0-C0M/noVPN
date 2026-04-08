from __future__ import annotations

import json
import os
import platform
import uuid
from pathlib import Path


class DeviceIdentityStore:
    def __init__(self, path: Path) -> None:
        self._path = path

    def device_id(self) -> str:
        payload = self._read_payload()
        stored = str(payload.get("device_id", "")).strip()
        if stored:
            return stored

        generated = "desktop-" + uuid.uuid4().hex
        self._write_payload({"device_id": generated})
        return generated

    def device_name(self) -> str:
        candidates = [
            os.environ.get("COMPUTERNAME", "").strip(),
            platform.node().strip(),
        ]
        parts: list[str] = []
        for candidate in candidates:
            if candidate and candidate not in parts:
                parts.append(candidate)
        if parts:
            return "Windows " + " ".join(parts)
        return "Windows desktop"

    def _read_payload(self) -> dict[str, object]:
        try:
            raw = self._path.read_text(encoding="utf-8")
        except FileNotFoundError:
            return {}
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            return {}
        return payload if isinstance(payload, dict) else {}

    def _write_payload(self, payload: dict[str, object]) -> None:
        self._path.parent.mkdir(parents=True, exist_ok=True)
        self._path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
