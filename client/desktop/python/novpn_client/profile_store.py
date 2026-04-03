from __future__ import annotations

import json
from dataclasses import asdict
from pathlib import Path

from .models import ClientProfile, LocalPorts, ObfuscationProfile, ServerProfile


class ProfileStore:
    def __init__(self, profile_path: Path) -> None:
        self._profile_path = profile_path

    @property
    def profile_path(self) -> Path:
        return self._profile_path

    def load(self) -> ClientProfile:
        payload = json.loads(self._profile_path.read_text(encoding="utf-8"))

        return ClientProfile(
            name=payload["name"],
            server=ServerProfile(**payload["server"]),
            local=LocalPorts(**payload.get("local", {})),
            obfuscation=ObfuscationProfile(**payload["obfuscation"]),
        )

    def save(self, profile: ClientProfile) -> None:
        payload = {
            "name": profile.name,
            "server": asdict(profile.server),
            "local": asdict(profile.local),
            "obfuscation": asdict(profile.obfuscation),
        }
        self._profile_path.parent.mkdir(parents=True, exist_ok=True)
        self._profile_path.write_text(
            json.dumps(payload, indent=2) + "\n",
            encoding="utf-8",
        )
