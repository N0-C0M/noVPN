from __future__ import annotations

import json
from dataclasses import replace
from pathlib import Path

from .models import (
    AppRoutingMode,
    ClientState,
    ConnectionMode,
    PatternMaskingStrategy,
    TrafficObfuscationStrategy,
)


class ClientStateStore:
    def __init__(self, path: Path) -> None:
        self._path = path

    def load(self, default_profile_key: str = "") -> ClientState:
        payload = self._read_payload()
        state = ClientState(
            bypass_ru=bool(payload.get("bypass_ru", True)),
            app_routing_mode=AppRoutingMode.from_storage(payload.get("app_routing_mode")),
            selected_apps=self._load_selected_apps(payload),
            traffic_strategy=TrafficObfuscationStrategy.from_storage(payload.get("traffic_strategy")),
            pattern_strategy=PatternMaskingStrategy.from_storage(payload.get("pattern_strategy")),
            connection_mode=ConnectionMode.from_storage(payload.get("connection_mode")),
            selected_profile_key=str(payload.get("selected_profile_key", "")).strip(),
            invite_code=str(payload.get("invite_code", "")).strip(),
            force_server_ip_mode=bool(payload.get("force_server_ip_mode", True)),
            device_id=str(payload.get("device_id", "")).strip(),
        )
        if not state.selected_profile_key and default_profile_key:
            state = replace(state, selected_profile_key=default_profile_key)
        return state

    def save(self, state: ClientState) -> ClientState:
        self._path.parent.mkdir(parents=True, exist_ok=True)
        document = {
            "bypass_ru": state.bypass_ru,
            "app_routing_mode": state.app_routing_mode.value,
            "selected_apps": sorted(set(state.selected_apps)),
            "traffic_strategy": state.traffic_strategy.value,
            "pattern_strategy": state.pattern_strategy.value,
            "connection_mode": state.connection_mode.value,
            "selected_profile_key": state.selected_profile_key,
            "invite_code": state.invite_code,
            "force_server_ip_mode": state.force_server_ip_mode,
            "device_id": state.device_id,
        }
        self._path.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")
        return state

    def update(self, state: ClientState, **changes: object) -> ClientState:
        return self.save(replace(state, **changes))

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

    def _load_selected_apps(self, payload: dict[str, object]) -> list[str]:
        raw = payload.get("selected_apps")
        if not isinstance(raw, list):
            return []
        result: list[str] = []
        for item in raw:
            candidate = str(item).strip()
            if candidate and candidate not in result:
                result.append(candidate)
        return result
