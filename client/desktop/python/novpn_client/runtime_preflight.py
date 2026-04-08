from __future__ import annotations

from dataclasses import dataclass

from .models import require_runtime_ready
from .profile_store import ProfileStore
from .runtime_layout import RuntimeLayout


@dataclass(slots=True)
class RuntimePreflightReport:
    is_ready: bool
    headline: str
    details: list[str]

    def require_ready(self) -> None:
        if not self.is_ready:
            raise RuntimeError(" ".join(self.details))


class RuntimePreflightChecker:
    def __init__(self, profile_store: ProfileStore, runtime_layout: RuntimeLayout) -> None:
        self._profile_store = profile_store
        self._runtime_layout = runtime_layout

    def evaluate(self, profile_key: str) -> RuntimePreflightReport:
        ready = True
        details: list[str] = []

        if not profile_key.strip():
            ready = False
            details.append("Сначала активируйте код или импортируйте профиль сервера.")
        else:
            try:
                require_runtime_ready(self._profile_store.load_by_key(profile_key))
            except Exception as exc:
                ready = False
                details.append(str(exc))
            else:
                details.append("Профиль клиента готов к запуску.")

        xray_binary = self._runtime_layout.xray_binary
        obfuscator_binary = self._runtime_layout.obfuscator_binary
        assets_dir = xray_binary.parent

        if xray_binary.is_file():
            details.append(f"Xray найден: {xray_binary.name}")
        else:
            ready = False
            details.append(f"Не найден Xray: {xray_binary}")

        if obfuscator_binary.is_file():
            details.append(f"Obfuscator найден: {obfuscator_binary.name}")
        else:
            ready = False
            details.append(f"Не найден obfuscator: {obfuscator_binary}")

        for asset_name in ("geoip.dat", "geosite.dat"):
            asset_path = assets_dir / asset_name
            if asset_path.is_file():
                details.append(f"Файл {asset_name} найден.")
            else:
                ready = False
                details.append(f"Не найден {asset_name}: {asset_path}")

        details.append("Десктопный клиент готовит локальный SOCKS/HTTP runtime без кнопки сохранения настроек.")
        return RuntimePreflightReport(
            is_ready=ready,
            headline="Среда готова" if ready else "Нужно исправить окружение",
            details=details,
        )
