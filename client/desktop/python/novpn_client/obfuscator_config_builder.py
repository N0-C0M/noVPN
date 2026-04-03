from __future__ import annotations

import json
from pathlib import Path

from .models import ClientProfile


class ObfuscatorConfigBuilder:
    def build(self, profile: ClientProfile, xray_config_path: Path) -> dict:
        return {
            "mode": "client",
            "seed": profile.obfuscation.seed,
            "remote": {
                "address": profile.server.address,
                "port": profile.server.port,
            },
            "integration": {
                "xrayConfigPath": str(xray_config_path),
                "expectedCli": "--config <path>",
            },
            "notes": [
                "This scaffold assumes the obfuscator accepts --config <path>.",
                "Adjust the runtime launcher if your module 1 binary uses a different CLI.",
            ],
        }

    def write(self, profile: ClientProfile, output_path: Path, xray_config_path: Path) -> Path:
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(
            json.dumps(self.build(profile, xray_config_path), indent=2) + "\n",
            encoding="utf-8",
        )
        return output_path
