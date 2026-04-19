from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path


class AppRoutingMode(str, Enum):
    EXCLUDE_SELECTED = "exclude_selected"
    ONLY_SELECTED = "only_selected"

    @classmethod
    def from_storage(cls, value: str | None) -> "AppRoutingMode":
        for item in cls:
            if item.value == value:
                return item
        return cls.EXCLUDE_SELECTED


class ConnectionMode(str, Enum):
    LOCAL_PROXY = "local_proxy"
    SYSTEM_TUNNEL = "system_tunnel"

    @classmethod
    def from_storage(cls, value: str | None) -> "ConnectionMode":
        for item in cls:
            if item.value == value:
                return item
        return cls.LOCAL_PROXY


class TrafficObfuscationStrategy(str, Enum):
    BALANCED = "balanced"
    CDN_MIMIC = "cdn_mimic"
    FRAGMENTED = "fragmented"
    MOBILE_MIX = "mobile_mix"
    TLS_BLEND = "tls_blend"

    @property
    def fingerprint(self) -> str:
        return {
            self.BALANCED: "chrome",
            self.CDN_MIMIC: "chrome",
            self.FRAGMENTED: "safari",
            self.MOBILE_MIX: "firefox",
            self.TLS_BLEND: "edge",
        }[self]

    @property
    def spider_xpath(self) -> str:
        return {
            self.BALANCED: "/",
            self.CDN_MIMIC: "/cdn-cgi/trace",
            self.FRAGMENTED: "/assets",
            self.MOBILE_MIX: "/generate_204",
            self.TLS_BLEND: "/favicon.ico",
        }[self]

    @classmethod
    def from_storage(cls, value: str | None) -> "TrafficObfuscationStrategy":
        for item in cls:
            if item.value == value:
                return item
        return cls.BALANCED


class PatternMaskingStrategy(str, Enum):
    STEADY = "steady"
    PULSE = "pulse"
    RANDOMIZED = "randomized"
    BURST_FADE = "burst_fade"
    QUIET_SWEEP = "quiet_sweep"

    @property
    def spider_xpath(self) -> str:
        return {
            self.STEADY: "/robots.txt",
            self.PULSE: "/cdn-cgi/trace",
            self.RANDOMIZED: "/assets/cache",
            self.BURST_FADE: "/generate_204",
            self.QUIET_SWEEP: "/favicon.ico",
        }[self]

    @property
    def jitter_window_ms(self) -> int:
        return {
            self.STEADY: 60,
            self.PULSE: 180,
            self.RANDOMIZED: 320,
            self.BURST_FADE: 420,
            self.QUIET_SWEEP: 240,
        }[self]

    @property
    def padding_profile(self) -> str:
        return {
            self.STEADY: "steady",
            self.PULSE: "pulse",
            self.RANDOMIZED: "randomized",
            self.BURST_FADE: "burst_fade",
            self.QUIET_SWEEP: "quiet_sweep",
        }[self]

    @classmethod
    def from_storage(cls, value: str | None) -> "PatternMaskingStrategy":
        for item in cls:
            if item.value == value:
                return item
        return cls.STEADY


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
    server_id: str = ""
    location_label: str = ""
    spider_x: str = "/"
    api_base: str = ""


@dataclass(slots=True)
class LocalPorts:
    socks_listen: str = "127.0.0.1"
    socks_port: int = 10808
    http_listen: str = "127.0.0.1"
    http_port: int = 10809


@dataclass(slots=True)
class ObfuscationProfile:
    seed: str
    traffic_strategy: TrafficObfuscationStrategy = TrafficObfuscationStrategy.BALANCED
    pattern_strategy: PatternMaskingStrategy = PatternMaskingStrategy.STEADY


@dataclass(slots=True)
class ClientProfile:
    name: str
    server: ServerProfile
    local: LocalPorts
    obfuscation: ObfuscationProfile


@dataclass(slots=True)
class ProfileOption:
    key: str
    name: str
    address: str
    server_name: str
    location_label: str
    is_imported: bool
    server_id: str = ""


@dataclass(slots=True)
class DesktopSettings:
    bypass_ru: bool
    app_routing_mode: AppRoutingMode
    selected_apps: list[str]
    traffic_strategy: TrafficObfuscationStrategy
    pattern_strategy: PatternMaskingStrategy
    connection_mode: ConnectionMode
    device_id: str
    output_path: Path
    network_interface_name: str = ""
    network_interface_ipv4: str = ""


@dataclass(slots=True)
class RuntimeStatus:
    running: bool
    xray_binary: Path
    obfuscator_binary: Path
    xray_log: Path
    obfuscator_log: Path
    detail: str


@dataclass(slots=True)
class ClientState:
    bypass_ru: bool = True
    app_routing_mode: AppRoutingMode = AppRoutingMode.EXCLUDE_SELECTED
    selected_apps: list[str] = field(default_factory=list)
    traffic_strategy: TrafficObfuscationStrategy = TrafficObfuscationStrategy.BALANCED
    pattern_strategy: PatternMaskingStrategy = PatternMaskingStrategy.STEADY
    connection_mode: ConnectionMode = ConnectionMode.LOCAL_PROXY
    selected_profile_key: str = ""
    invite_code: str = ""
    force_server_ip_mode: bool = True
    device_id: str = ""


def with_runtime_strategies(
    profile: ClientProfile,
    traffic_strategy: TrafficObfuscationStrategy,
    pattern_strategy: PatternMaskingStrategy,
) -> ClientProfile:
    return ClientProfile(
        name=profile.name,
        server=profile.server,
        local=profile.local,
        obfuscation=ObfuscationProfile(
            seed=profile.obfuscation.seed,
            traffic_strategy=traffic_strategy,
            pattern_strategy=pattern_strategy,
        ),
    )


def require_runtime_ready(profile: ClientProfile) -> None:
    invalid_fields: list[str] = []
    if not profile.server.address.strip():
        invalid_fields.append("address")
    if not profile.server.uuid.strip() or profile.server.uuid == "00000000-0000-4000-8000-000000000000":
        invalid_fields.append("uuid")
    if not profile.server.server_name.strip():
        invalid_fields.append("server_name")
    if not profile.server.public_key.strip() or profile.server.public_key.startswith("REPLACE_WITH_"):
        invalid_fields.append("public_key")
    if not profile.server.short_id.strip() or profile.server.short_id.startswith("REPLACE_WITH_"):
        invalid_fields.append("short_id")

    if invalid_fields:
        joined = ", ".join(invalid_fields)
        raise RuntimeError(
            "Import or activate a real server profile before connecting. "
            f"Missing fields: {joined}."
        )
