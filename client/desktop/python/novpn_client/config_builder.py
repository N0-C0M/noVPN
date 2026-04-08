from __future__ import annotations

import json
from pathlib import Path

from .models import ClientProfile, DesktopSettings
from .session_obfuscation import SessionObfuscationPlan, SessionObfuscationPlanner


class XrayConfigBuilder:
    def build(
        self,
        profile: ClientProfile,
        settings: DesktopSettings,
        session_plan: SessionObfuscationPlan | None = None,
    ) -> dict:
        effective_plan = session_plan or SessionObfuscationPlanner.build(
            profile=profile,
            device_id=settings.device_id or "desktop-preview",
        )
        rules: list[dict] = [
            {
                "type": "field",
                "ip": ["geoip:private"],
                "outboundTag": "direct",
                "ruleTag": "private-direct",
            },
            {
                "type": "field",
                "process": ["self/", "xray/"],
                "outboundTag": "direct",
                "ruleTag": "runtime-self-direct",
            },
        ]

        if settings.bypass_ru:
            rules.extend(
                [
                    {
                        "type": "field",
                        "domain": ["domain:ru", "domain:su", "domain:xn--p1ai"],
                        "outboundTag": "direct",
                        "ruleTag": "ru-domain-direct",
                    },
                    {
                        "type": "field",
                        "ip": ["geoip:ru"],
                        "outboundTag": "direct",
                        "ruleTag": "ru-ip-direct",
                    },
                ]
            )

        selected_processes = self._selected_processes(settings.selected_apps)
        if selected_processes:
            rules.append(
                {
                    "type": "field",
                    "process": selected_processes,
                    "outboundTag": (
                        "direct"
                        if settings.app_routing_mode.value == "exclude_selected"
                        else "proxy"
                    ),
                    "ruleTag": (
                        "desktop-excluded-apps-direct"
                        if settings.app_routing_mode.value == "exclude_selected"
                        else "desktop-selected-apps-proxy"
                    ),
                }
            )

        default_outbound = "proxy"
        if selected_processes and settings.app_routing_mode.value == "only_selected":
            default_outbound = "direct"

        rules.append(
            {
                "type": "field",
                "network": "tcp,udp",
                "outboundTag": default_outbound,
                "ruleTag": f"default-{default_outbound}",
            }
        )

        return {
            "log": {"loglevel": "warning"},
            "dns": {"servers": ["1.1.1.1", "8.8.8.8"]},
            "inbounds": [
                {
                    "tag": "socks-in",
                    "listen": profile.local.socks_listen,
                    "port": profile.local.socks_port,
                    "protocol": "socks",
                    "settings": {"udp": True},
                    "sniffing": {
                        "enabled": True,
                        "destOverride": ["http", "tls", "quic"],
                        "routeOnly": True,
                    },
                },
                {
                    "tag": "http-in",
                    "listen": profile.local.http_listen,
                    "port": profile.local.http_port,
                    "protocol": "http",
                    "sniffing": {
                        "enabled": True,
                        "destOverride": ["http", "tls"],
                        "routeOnly": True,
                    },
                },
            ],
            "outbounds": [
                {
                    "tag": "proxy",
                    "protocol": "vless",
                    "settings": {
                        "vnext": [
                            {
                                "address": profile.server.address,
                                "port": profile.server.port,
                                "users": [
                                    {
                                        "id": profile.server.uuid,
                                        "encryption": "none",
                                        "flow": profile.server.flow,
                                    }
                                ],
                            }
                        ]
                    },
                    "streamSettings": {
                        "network": "tcp",
                        "security": "reality",
                        "realitySettings": {
                            "serverName": profile.server.server_name,
                            "fingerprint": effective_plan.selected_fingerprint,
                            "publicKey": profile.server.public_key,
                            "shortId": profile.server.short_id,
                            "spiderX": effective_plan.selected_spider_x,
                        },
                    },
                },
                {"tag": "direct", "protocol": "freedom"},
                {"tag": "block", "protocol": "blackhole"},
                {"tag": "dns-out", "protocol": "dns"},
            ],
            "routing": {
                "domainStrategy": "IPIfNonMatch",
                "rules": rules,
            },
            "observatory": {
                "subjectSelector": ["proxy"],
                "probeURL": "https://www.gstatic.com/generate_204",
                "probeInterval": "5m",
            },
        }

    def write(
        self,
        profile: ClientProfile,
        settings: DesktopSettings,
        session_plan: SessionObfuscationPlan | None = None,
    ) -> Path:
        document = self.build(profile, settings, session_plan)
        settings.output_path.parent.mkdir(parents=True, exist_ok=True)
        settings.output_path.write_text(
            json.dumps(document, indent=2) + "\n",
            encoding="utf-8",
        )
        return settings.output_path

    def _selected_processes(self, selected_apps: list[str]) -> list[str]:
        result: list[str] = []
        for raw_path in selected_apps:
            normalized = str(raw_path).strip()
            if not normalized:
                continue
            process_name = Path(normalized).stem.strip()
            if process_name and process_name not in result:
                result.append(process_name)
            normalized_path = normalized.replace("\\", "/")
            if "/" in normalized_path and normalized_path not in result:
                result.append(normalized_path)
        return result
