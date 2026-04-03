from __future__ import annotations

import json
from dataclasses import asdict
from pathlib import Path

from .models import ClientProfile, LocalPorts, ObfuscationProfile, ProfileOption, ServerProfile


class ProfileStore:
    def __init__(self, profile_path: Path) -> None:
        self._profile_path = profile_path

    @property
    def profile_path(self) -> Path:
        return self._profile_path

    def available_profiles(self) -> list[ProfileOption]:
        directory = self._profile_path.parent
        profile_files = sorted(directory.glob("*.profile.json"))
        if not profile_files and self._profile_path.exists():
            profile_files = [self._profile_path]

        options: list[ProfileOption] = []
        for path in profile_files:
            profile = self.load(path)
            options.append(
                ProfileOption(
                    key=path.name,
                    name=profile.name,
                    address=f"{profile.server.address}:{profile.server.port}",
                    server_name=profile.server.server_name,
                )
            )
        return options

    def load(self, profile_path: Path | None = None) -> ClientProfile:
        target_path = profile_path or self._profile_path
        payload = json.loads(target_path.read_text(encoding="utf-8"))

        return ClientProfile(
            name=payload["name"],
            server=ServerProfile(**payload["server"]),
            local=LocalPorts(**payload.get("local", {})),
            obfuscation=ObfuscationProfile(**payload["obfuscation"]),
        )

    def load_by_key(self, profile_key: str) -> ClientProfile:
        return self.load(self._profile_path.parent / profile_key)

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
