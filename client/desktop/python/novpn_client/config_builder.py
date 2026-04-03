from __future__ import annotations

import json
from pathlib import Path

from .models import ClientProfile, DesktopSettings


class XrayConfigBuilder:
    def build(self, profile: ClientProfile, settings: DesktopSettings) -> dict:
        rules: list[dict] = [
            {
                "type": "field",
                "ip": ["geoip:private"],
                "outboundTag": "direct",
                "ruleTag": "private-direct",
            }
        ]

        if settings.bypass_ru:
            rules.extend(
                [
                    {
                        "type": "field",
                        "domain": ["ext:geosite.dat:ru"],
                        "outboundTag": "direct",
                        "ruleTag": "ru-domain-direct",
                    },
                    {
                        "type": "field",
                        "ip": ["ext:geoip.dat:ru"],
                        "outboundTag": "direct",
                        "ruleTag": "ru-ip-direct",
                    },
                ]
            )

        if settings.excluded_apps:
            rules.append(
                {
                    "type": "field",
                    "process": settings.excluded_apps,
                    "outboundTag": "direct",
                    "ruleTag": "desktop-excluded-apps-direct",
                }
            )

        rules.append(
            {
                "type": "field",
                "network": "tcp,udp",
                "outboundTag": "proxy",
                "ruleTag": "default-proxy",
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
                            "fingerprint": profile.server.fingerprint,
                            "publicKey": profile.server.public_key,
                            "shortId": profile.server.short_id,
                            "spiderX": profile.server.spider_x,
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

    def write(self, profile: ClientProfile, settings: DesktopSettings) -> Path:
        document = self.build(profile, settings)
        settings.output_path.parent.mkdir(parents=True, exist_ok=True)
        settings.output_path.write_text(
            json.dumps(document, indent=2) + "\n",
            encoding="utf-8",
        )
        return settings.output_path
