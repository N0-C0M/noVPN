from __future__ import annotations

import json
from dataclasses import dataclass
from enum import Enum
from urllib import error, parse, request


class CodeRedeemKind(str, Enum):
    INVITE = "invite"
    PROMO = "promo"


@dataclass(slots=True)
class CodeRedeemResult:
    kind: CodeRedeemKind
    profile_payload: str = ""
    profile_payloads: list[str] | None = None
    profile_name: str = ""
    bonus_bytes: int = 0
    activation_mode: str = ""

    def __post_init__(self) -> None:
        if self.profile_payloads is None:
            self.profile_payloads = []


class InviteRedeemer:
    def redeem(
        self,
        server_address: str,
        invite_code: str,
        device_id: str,
        device_name: str,
        api_base: str = "",
    ) -> CodeRedeemResult:
        normalized_address = self._normalize_server_address(server_address)
        normalized_api_base = self._normalize_api_base(server_address, api_base)
        normalized_code = invite_code.strip()
        if not normalized_address and not normalized_api_base:
            raise RuntimeError("РќРµС‚ Р°РґСЂРµСЃР° СЃРµСЂРІРµСЂР° РґР»СЏ Р°РєС‚РёРІР°С†РёРё РєРѕРґР°.")
        if not normalized_code:
            raise RuntimeError("Р’РІРµРґРёС‚Рµ РєР»СЋС‡ РёР»Рё РїСЂРѕРјРѕРєРѕРґ.")

        endpoint = f"{normalized_api_base}/redeem/{parse.quote(normalized_code)}"
        payload = {
            "device_id": device_id.strip(),
            "device_name": device_name.strip(),
        }
        response = self._post_json(endpoint, payload)
        kind = str(response.get("kind", "")).strip().lower()
        if kind == CodeRedeemKind.INVITE.value:
            payloads = self._extract_profile_payloads(response)
            if not payloads:
                raise RuntimeError("РЎРµСЂРІРµСЂ РЅРµ РІРµСЂРЅСѓР» РїСЂРѕС„РёР»СЊ РєР»РёРµРЅС‚Р°.")
            return CodeRedeemResult(
                kind=CodeRedeemKind.INVITE,
                profile_payload=payloads[0],
                profile_payloads=payloads,
                profile_name=self._extract_profile_name(response),
            )
        if kind == CodeRedeemKind.PROMO.value:
            payloads = self._extract_profile_payloads(response)
            return CodeRedeemResult(
                kind=CodeRedeemKind.PROMO,
                profile_payload=payloads[0] if payloads else "",
                profile_payloads=payloads,
                profile_name=self._extract_profile_name(response),
                bonus_bytes=int(response.get("bonus_bytes", 0) or 0),
                activation_mode=str(response.get("activation_mode", "")).strip().lower(),
            )
        raise RuntimeError("РЎРµСЂРІРµСЂ РІРµСЂРЅСѓР» РїСѓСЃС‚РѕР№ РёР»Рё РЅРµРїРѕРЅСЏС‚РЅС‹Р№ РѕС‚РІРµС‚ РЅР° Р°РєС‚РёРІР°С†РёСЋ РєРѕРґР°.")

    def disconnect(
        self,
        server_address: str,
        device_id: str,
        device_name: str,
        client_uuid: str,
        api_base: str = "",
    ) -> None:
        normalized_address = self._normalize_server_address(server_address)
        normalized_api_base = self._normalize_api_base(server_address, api_base)
        if not normalized_address and not normalized_api_base:
            raise RuntimeError("РќРµС‚ Р°РґСЂРµСЃР° СЃРµСЂРІРµСЂР° РґР»СЏ РѕС‚РІСЏР·РєРё СѓСЃС‚СЂРѕР№СЃС‚РІР°.")
        if not device_id.strip():
            raise RuntimeError("РќРµ РЅР°Р№РґРµРЅ РёРґРµРЅС‚РёС„РёРєР°С‚РѕСЂ СѓСЃС‚СЂРѕР№СЃС‚РІР°.")
        if not client_uuid.strip():
            raise RuntimeError("РќРµ РЅР°Р№РґРµРЅ UUID РєР»РёРµРЅС‚Р°.")

        endpoint = f"{normalized_api_base}/disconnect"
        payload = {
            "device_id": device_id.strip(),
            "device_name": device_name.strip(),
            "client_uuid": client_uuid.strip(),
        }
        self._post_json(endpoint, payload)

    def _post_json(self, url: str, payload: dict[str, object]) -> dict[str, object]:
        encoded_payload = json.dumps(payload).encode("utf-8")
        http_request = request.Request(
            url,
            data=encoded_payload,
            headers={
                "Content-Type": "application/json; charset=utf-8",
                "Accept": "application/json",
            },
            method="POST",
        )
        try:
            with request.urlopen(http_request, timeout=15) as response:
                body = response.read().decode("utf-8", errors="replace").strip()
        except error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace").strip()
            raise RuntimeError(body or f"РЎРµСЂРІРµСЂ РІРµСЂРЅСѓР» HTTP {exc.code}.") from exc
        except error.URLError as exc:
            raise RuntimeError(f"РќРµ СѓРґР°Р»РѕСЃСЊ РїРѕРґРєР»СЋС‡РёС‚СЊСЃСЏ Рє СЃРµСЂРІРµСЂСѓ: {exc.reason}.") from exc

        try:
            decoded = json.loads(body)
        except json.JSONDecodeError as exc:
            raise RuntimeError(body or "РЎРµСЂРІРµСЂ РІРµСЂРЅСѓР» РїСѓСЃС‚РѕР№ РѕС‚РІРµС‚.") from exc
        if not isinstance(decoded, dict):
            raise RuntimeError("РЎРµСЂРІРµСЂ РІРµСЂРЅСѓР» РѕС‚РІРµС‚ РІ РЅРµРѕР¶РёРґР°РЅРЅРѕРј С„РѕСЂРјР°С‚Рµ.")
        return decoded

    def _normalize_server_address(self, server_address: str) -> str:
        return server_address.strip().strip("/").removeprefix("http://").removeprefix("https://")

    def _normalize_api_base(self, server_address: str, api_base: str) -> str:
        normalized_api_base = api_base.strip().strip("/")
        if normalized_api_base:
            if normalized_api_base.startswith(("http://", "https://")):
                return normalized_api_base
            return f"http://{normalized_api_base}"

        normalized_address = self._normalize_server_address(server_address)
        if not normalized_address:
            return ""
        return f"http://{normalized_address}/admin"

    def _extract_profile_payloads(self, response: dict[str, object]) -> list[str]:
        payloads: list[str] = []
        raw_payloads = response.get("client_profiles_yaml")
        if isinstance(raw_payloads, list):
            for item in raw_payloads:
                if not isinstance(item, str):
                    continue
                candidate = item.strip()
                if candidate:
                    payloads.append(candidate)
        if payloads:
            return payloads

        fallback = str(response.get("client_profile_yaml", "")).strip()
        if fallback:
            payloads.append(fallback)
            return payloads

        raw_profiles = response.get("client_profiles")
        if isinstance(raw_profiles, list):
            for item in raw_profiles:
                if not isinstance(item, dict):
                    continue
                payload = self._build_canonical_profile_payload(item)
                if payload:
                    payloads.append(payload)
        return payloads

    def _extract_profile_name(self, response: dict[str, object]) -> str:
        client_profile = response.get("client_profile")
        if isinstance(client_profile, dict):
            name = str(client_profile.get("name", "")).strip()
            if name:
                return name

        raw_profiles = response.get("client_profiles")
        if isinstance(raw_profiles, list):
            for item in raw_profiles:
                if not isinstance(item, dict):
                    continue
                name = str(item.get("name", "")).strip()
                if name:
                    return name
        return ""

    def _build_canonical_profile_payload(self, source: dict[str, object]) -> str:
        name = self._pick_string(source, "name", "Name") or "Imported Reality Profile"
        address = self._pick_string(source, "address", "Address")
        port = self._pick_int(source, "port", "Port")
        uuid_value = self._pick_string(source, "uuid", "UUID")
        flow = self._pick_string(source, "flow", "Flow") or "xtls-rprx-vision"
        server_name = self._pick_string(source, "server_name", "ServerName")
        fingerprint = self._pick_string(source, "fingerprint", "Fingerprint") or "chrome"
        public_key = self._pick_string(source, "public_key", "PublicKey")
        short_id = self._pick_string(source, "short_id", "ShortID") or self._pick_first_string(
            source,
            "short_ids",
            "ShortIDs",
        )
        spider_x = self._pick_string(source, "spider_x", "SpiderX") or "/"

        if not address or port <= 0 or not uuid_value or not server_name or not public_key or not short_id:
            return ""

        payload = {
            "name": name,
            "server": {
                "server_id": self._pick_string(source, "server_id", "ServerID"),
                "address": address,
                "port": port,
                "uuid": uuid_value,
                "flow": flow,
                "server_name": server_name,
                "fingerprint": fingerprint,
                "public_key": public_key,
                "short_id": short_id,
                "location_label": self._pick_string(source, "location_label", "Location"),
                "spider_x": spider_x,
                "api_base": self._pick_string(source, "api_base", "APIBase"),
            },
        }
        return json.dumps(payload)

    def _pick_string(self, source: dict[str, object], *keys: str) -> str:
        for key in keys:
            value = str(source.get(key, "")).strip()
            if value:
                return value
        return ""

    def _pick_int(self, source: dict[str, object], *keys: str) -> int:
        for key in keys:
            raw_value = source.get(key)
            try:
                value = int(raw_value or 0)
            except (TypeError, ValueError):
                continue
            if value > 0:
                return value
        return 0

    def _pick_first_string(self, source: dict[str, object], *keys: str) -> str:
        for key in keys:
            raw_values = source.get(key)
            if not isinstance(raw_values, list):
                continue
            for item in raw_values:
                value = str(item).strip()
                if value:
                    return value
        return ""
