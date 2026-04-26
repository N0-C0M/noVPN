from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from novpn_client.models import (
    ClientProfile,
    LocalPorts,
    ObfuscationProfile,
    ServerProfile,
)


def build_profile(local: LocalPorts | None = None) -> ClientProfile:
    return ClientProfile(
        name="Test profile",
        server=ServerProfile(
            address="198.51.100.10",
            port=443,
            uuid="12345678-1234-4234-8234-1234567890ab",
            flow="xtls-rprx-vision",
            server_name="vpn.example.test",
            fingerprint="chrome",
            public_key="test-public-key",
            short_id="abcd1234",
        ),
        local=local or LocalPorts(),
        obfuscation=ObfuscationProfile(seed="novpn-seed-test"),
    )
