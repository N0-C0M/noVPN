from __future__ import annotations

import os
from pathlib import Path


class AppCatalogService:
    _KNOWN_EXECUTABLES = [
        r"C:\Program Files\Telegram Desktop\Telegram.exe",
        r"C:\Program Files\Yandex\YandexBrowser\Application\browser.exe",
        r"C:\Program Files\Mozilla Firefox\firefox.exe",
        r"C:\Program Files\Google\Chrome\Application\chrome.exe",
        r"C:\Users\%USERNAME%\AppData\Local\Programs\Opera\opera.exe",
    ]

    def list_candidates(self, extra_paths: list[str] | None = None) -> list[str]:
        result: list[str] = []
        for raw_path in [*self._KNOWN_EXECUTABLES, *(extra_paths or [])]:
            normalized = self.normalize_executable(raw_path)
            if normalized and normalized not in result:
                result.append(normalized)
        return result

    def normalize_executable(self, raw_path: str | Path) -> str:
        path = Path(os.path.expandvars(str(raw_path))).expanduser()
        if path.suffix.lower() != ".exe":
            return ""
        if not path.exists():
            return ""
        return str(path.resolve())
