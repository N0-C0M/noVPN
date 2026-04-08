from __future__ import annotations

import json
from pathlib import Path

from .models import ClientProfile
from .session_obfuscation import SessionObfuscationPlan, SessionObfuscationPlanner


class ObfuscatorConfigBuilder:
    def build(
        self,
        profile: ClientProfile,
        xray_config_path: Path,
        device_id: str = "",
        session_plan: SessionObfuscationPlan | None = None,
    ) -> dict:
        effective_plan = session_plan or SessionObfuscationPlanner.build(
            profile=profile,
            device_id=device_id or "desktop-preview",
        )
        return {
            "mode": "client",
            "seed": profile.obfuscation.seed,
            "traffic_strategy": profile.obfuscation.traffic_strategy.value,
            "pattern_strategy": profile.obfuscation.pattern_strategy.value,
            "remote": {
                "address": profile.server.address,
                "port": profile.server.port,
            },
            "integration": {
                "xrayConfigPath": str(xray_config_path),
                "expectedCli": "--config <path>",
            },
            "session": {
                "nonce": effective_plan.session_nonce,
                "rotation_bucket": effective_plan.rotation_bucket,
                "selected_fingerprint": effective_plan.selected_fingerprint,
                "selected_spider_x": effective_plan.selected_spider_x,
                "fingerprint_pool": effective_plan.fingerprint_pool,
                "cover_path_pool": effective_plan.cover_path_pool,
            },
            "pattern_tuning": {
                "padding_profile": effective_plan.padding_profile,
                "jitter_window_ms": effective_plan.jitter_window_ms,
                "padding_min_bytes": effective_plan.padding_min_bytes,
                "padding_max_bytes": effective_plan.padding_max_bytes,
                "burst_interval_min_ms": effective_plan.burst_interval_min_ms,
                "burst_interval_max_ms": effective_plan.burst_interval_max_ms,
                "idle_gap_min_ms": effective_plan.idle_gap_min_ms,
                "idle_gap_max_ms": effective_plan.idle_gap_max_ms,
            },
            "notes": [
                "This scaffold assumes the obfuscator accepts --config <path>.",
                "Adjust the runtime launcher if your module 1 binary uses a different CLI.",
            ],
        }

    def write(
        self,
        profile: ClientProfile,
        output_path: Path,
        xray_config_path: Path,
        device_id: str = "",
        session_plan: SessionObfuscationPlan | None = None,
    ) -> Path:
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(
            json.dumps(
                self.build(profile, xray_config_path, device_id, session_plan),
                indent=2,
            ) + "\n",
            encoding="utf-8",
        )
        return output_path
