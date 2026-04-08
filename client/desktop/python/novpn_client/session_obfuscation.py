from __future__ import annotations

import hashlib
import time
from dataclasses import dataclass

from .models import ClientProfile, PatternMaskingStrategy, TrafficObfuscationStrategy


@dataclass(slots=True)
class SessionObfuscationPlan:
    rotation_bucket: int
    session_nonce: str
    selected_fingerprint: str
    selected_spider_x: str
    fingerprint_pool: list[str]
    cover_path_pool: list[str]
    padding_profile: str
    jitter_window_ms: int
    padding_min_bytes: int
    padding_max_bytes: int
    burst_interval_min_ms: int
    burst_interval_max_ms: int
    idle_gap_min_ms: int
    idle_gap_max_ms: int


class SessionObfuscationPlanner:
    _ROTATION_INTERVAL_MS = 20 * 60 * 1000

    @classmethod
    def build(
        cls,
        profile: ClientProfile,
        device_id: str,
        now_ms: int | None = None,
    ) -> SessionObfuscationPlan:
        effective_now_ms = now_ms if now_ms is not None else int(time.time() * 1000)
        rotation_bucket = effective_now_ms // cls._ROTATION_INTERVAL_MS
        digest = hashlib.sha256(
            (
                f"{profile.obfuscation.seed}|{device_id.strip()}|{profile.server.short_id}|"
                f"{profile.server.address}|{profile.server.server_name}|{rotation_bucket}|"
                f"{profile.obfuscation.traffic_strategy.value}|{profile.obfuscation.pattern_strategy.value}"
            ).encode("utf-8")
        ).digest()
        digest_hex = digest.hex()

        fingerprint_pool = cls._fingerprint_pool(profile)
        selected_fingerprint = fingerprint_pool[digest[0] % len(fingerprint_pool)]

        cover_path_pool = cls._cover_path_pool(profile, digest_hex, selected_fingerprint)
        selected_spider_x = cover_path_pool[digest[1] % len(cover_path_pool)]

        padding_min_bytes, padding_max_bytes = cls._padding_range(profile.obfuscation.pattern_strategy, digest)
        burst_min_ms, burst_max_ms = cls._burst_range(profile.obfuscation.pattern_strategy, digest)
        idle_min_ms, idle_max_ms = cls._idle_range(profile.obfuscation.pattern_strategy, digest)
        padding_profile = cls._padding_profile(profile.obfuscation.pattern_strategy, digest)
        jitter_window_ms = profile.obfuscation.pattern_strategy.jitter_window_ms + (digest[2] % 120)

        return SessionObfuscationPlan(
            rotation_bucket=rotation_bucket,
            session_nonce=digest_hex[:16],
            selected_fingerprint=selected_fingerprint,
            selected_spider_x=selected_spider_x,
            fingerprint_pool=fingerprint_pool,
            cover_path_pool=cover_path_pool,
            padding_profile=padding_profile,
            jitter_window_ms=jitter_window_ms,
            padding_min_bytes=padding_min_bytes,
            padding_max_bytes=padding_max_bytes,
            burst_interval_min_ms=burst_min_ms,
            burst_interval_max_ms=burst_max_ms,
            idle_gap_min_ms=idle_min_ms,
            idle_gap_max_ms=idle_max_ms,
        )

    @classmethod
    def _fingerprint_pool(cls, profile: ClientProfile) -> list[str]:
        base = profile.server.fingerprint.strip() or "chrome"
        strategy = profile.obfuscation.traffic_strategy
        variants = {
            TrafficObfuscationStrategy.BALANCED: [base, "chrome", "firefox", "edge"],
            TrafficObfuscationStrategy.CDN_MIMIC: ["chrome", "edge", base],
            TrafficObfuscationStrategy.FRAGMENTED: ["safari", "chrome", "firefox"],
            TrafficObfuscationStrategy.MOBILE_MIX: ["firefox", "safari", "chrome"],
            TrafficObfuscationStrategy.TLS_BLEND: ["edge", "chrome", "safari"],
        }[strategy]
        return cls._dedupe([value.strip() for value in variants if value.strip()])

    @classmethod
    def _cover_path_pool(
        cls,
        profile: ClientProfile,
        digest_hex: str,
        selected_fingerprint: str,
    ) -> list[str]:
        short_suffix = profile.server.short_id.strip()[-4:] or "edge"
        seed_suffix = digest_hex[:6]
        fingerprint_hint = selected_fingerprint[:3].lower()
        pattern = profile.obfuscation.pattern_strategy
        traffic = profile.obfuscation.traffic_strategy
        base_paths = [
            profile.server.spider_x.strip() or "/",
            pattern.spider_xpath,
            traffic.spider_xpath,
            f"{pattern.spider_xpath}/{short_suffix}",
            f"{pattern.spider_xpath}?v={seed_suffix}",
        ]
        if pattern == PatternMaskingStrategy.QUIET_SWEEP:
            base_paths.append(f"{pattern.spider_xpath}?fp={fingerprint_hint}&r={seed_suffix}")
        elif pattern == PatternMaskingStrategy.RANDOMIZED:
            base_paths.append(f"{pattern.spider_xpath}/{seed_suffix}")
        elif pattern == PatternMaskingStrategy.BURST_FADE:
            base_paths.append(f"{pattern.spider_xpath}?burst={seed_suffix}")
        elif pattern == PatternMaskingStrategy.PULSE:
            base_paths.append(f"{pattern.spider_xpath}?pulse={seed_suffix}")

        normalized: list[str] = []
        for path in base_paths:
            candidate = path.strip() or "/"
            if not candidate.startswith("/"):
                candidate = "/" + candidate.lstrip("/")
            if candidate not in normalized:
                normalized.append(candidate)
        return normalized

    @classmethod
    def _padding_range(
        cls,
        strategy: PatternMaskingStrategy,
        digest: bytes,
    ) -> tuple[int, int]:
        base_ranges = {
            PatternMaskingStrategy.STEADY: (96, 224),
            PatternMaskingStrategy.PULSE: (160, 448),
            PatternMaskingStrategy.RANDOMIZED: (192, 896),
            PatternMaskingStrategy.BURST_FADE: (256, 1280),
            PatternMaskingStrategy.QUIET_SWEEP: (72, 192),
        }
        min_bytes, max_bytes = base_ranges[strategy]
        min_bytes += digest[3] % 48
        max_bytes += digest[4] % 192
        if max_bytes <= min_bytes:
            max_bytes = min_bytes + 128
        return min_bytes, max_bytes

    @classmethod
    def _burst_range(
        cls,
        strategy: PatternMaskingStrategy,
        digest: bytes,
    ) -> tuple[int, int]:
        base_ranges = {
            PatternMaskingStrategy.STEADY: (1800, 3200),
            PatternMaskingStrategy.PULSE: (900, 2200),
            PatternMaskingStrategy.RANDOMIZED: (700, 2600),
            PatternMaskingStrategy.BURST_FADE: (500, 1800),
            PatternMaskingStrategy.QUIET_SWEEP: (2400, 4200),
        }
        min_ms, max_ms = base_ranges[strategy]
        min_ms += digest[5] % 220
        max_ms += digest[6] % 480
        if max_ms <= min_ms:
            max_ms = min_ms + 400
        return min_ms, max_ms

    @classmethod
    def _idle_range(
        cls,
        strategy: PatternMaskingStrategy,
        digest: bytes,
    ) -> tuple[int, int]:
        base_ranges = {
            PatternMaskingStrategy.STEADY: (2600, 5200),
            PatternMaskingStrategy.PULSE: (1500, 3600),
            PatternMaskingStrategy.RANDOMIZED: (900, 5000),
            PatternMaskingStrategy.BURST_FADE: (700, 3200),
            PatternMaskingStrategy.QUIET_SWEEP: (3200, 6400),
        }
        min_ms, max_ms = base_ranges[strategy]
        min_ms += digest[7] % 300
        max_ms += digest[8] % 720
        if max_ms <= min_ms:
            max_ms = min_ms + 600
        return min_ms, max_ms

    @classmethod
    def _padding_profile(cls, strategy: PatternMaskingStrategy, digest: bytes) -> str:
        suffixes = ["light", "mixed", "dense"]
        return f"{strategy.padding_profile}-{suffixes[digest[9] % len(suffixes)]}"

    @staticmethod
    def _dedupe(values: list[str]) -> list[str]:
        result: list[str] = []
        for item in values:
            if item and item not in result:
                result.append(item)
        return result
