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

    def list_candidates(self) -> list[str]:
        result: list[str] = []
        for raw_path in self._KNOWN_EXECUTABLES:
            path = Path(os.path.expandvars(raw_path))
            if path.exists():
                result.append(str(path))
        return result
