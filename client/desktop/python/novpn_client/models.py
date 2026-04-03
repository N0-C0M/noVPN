from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(slots=True)
class ServerProfile:
    address: str
    port: int
    uuid: str
    flow: str
    server_name: str
    fingerprint: str
    public_key: str
    short_id: str
    spider_x: str = "/"


@dataclass(slots=True)
class LocalPorts:
    socks_listen: str = "127.0.0.1"
    socks_port: int = 10808
    http_listen: str = "127.0.0.1"
    http_port: int = 10809


@dataclass(slots=True)
class ObfuscationProfile:
    seed: str


@dataclass(slots=True)
class ClientProfile:
    name: str
    server: ServerProfile
    local: LocalPorts
    obfuscation: ObfuscationProfile


@dataclass(slots=True)
class DesktopSettings:
    bypass_ru: bool
    excluded_apps: list[str]
    output_path: Path
