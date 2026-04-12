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
    profile_name: str = ""
    bonus_bytes: int = 0
    activation_mode: str = ""


class InviteRedeemer:
    def redeem(
        self,
        server_address: str,
        invite_code: str,
        device_id: str,
        device_name: str,
    ) -> CodeRedeemResult:
        normalized_address = self._normalize_server_address(server_address)
        normalized_code = invite_code.strip()
        if not normalized_address:
            raise RuntimeError("Нет адреса сервера для активации кода.")
        if not normalized_code:
            raise RuntimeError("Введите ключ или промокод.")

        endpoint = f"http://{normalized_address}/admin/redeem/{parse.quote(normalized_code)}"
        payload = {
            "device_id": device_id.strip(),
            "device_name": device_name.strip(),
        }
        response = self._post_json(endpoint, payload)
        kind = str(response.get("kind", "")).strip().lower()
        if kind == CodeRedeemKind.INVITE.value:
            profile_payload = str(response.get("client_profile_yaml", "")).strip()
            if not profile_payload:
                raise RuntimeError("Сервер не вернул профиль клиента.")
            profile_name = ""
            if isinstance(response.get("client_profile"), dict):
                profile_name = str(response["client_profile"].get("name", "")).strip()
            return CodeRedeemResult(
                kind=CodeRedeemKind.INVITE,
                profile_payload=profile_payload,
                profile_name=profile_name,
            )
        if kind == CodeRedeemKind.PROMO.value:
            profile_payload = str(response.get("client_profile_yaml", "")).strip()
            profile_name = ""
            if isinstance(response.get("client_profile"), dict):
                profile_name = str(response["client_profile"].get("name", "")).strip()
            return CodeRedeemResult(
                kind=CodeRedeemKind.PROMO,
                profile_payload=profile_payload,
                profile_name=profile_name,
                bonus_bytes=int(response.get("bonus_bytes", 0) or 0),
                activation_mode=str(response.get("activation_mode", "")).strip().lower(),
            )
        raise RuntimeError("Сервер вернул пустой или непонятный ответ на активацию кода.")

    def disconnect(
        self,
        server_address: str,
        device_id: str,
        device_name: str,
        client_uuid: str,
    ) -> None:
        normalized_address = self._normalize_server_address(server_address)
        if not normalized_address:
            raise RuntimeError("Нет адреса сервера для отвязки устройства.")
        if not device_id.strip():
            raise RuntimeError("Не найден идентификатор устройства.")
        if not client_uuid.strip():
            raise RuntimeError("Не найден UUID клиента.")

        endpoint = f"http://{normalized_address}/admin/disconnect"
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
            raise RuntimeError(body or f"Сервер вернул HTTP {exc.code}.") from exc
        except error.URLError as exc:
            raise RuntimeError(f"Не удалось подключиться к серверу: {exc.reason}.") from exc

        try:
            decoded = json.loads(body)
        except json.JSONDecodeError as exc:
            raise RuntimeError(body or "Сервер вернул пустой ответ.") from exc
        if not isinstance(decoded, dict):
            raise RuntimeError("Сервер вернул ответ в неожиданном формате.")
        return decoded

    def _normalize_server_address(self, server_address: str) -> str:
        return server_address.strip().strip("/").removeprefix("http://").removeprefix("https://")
