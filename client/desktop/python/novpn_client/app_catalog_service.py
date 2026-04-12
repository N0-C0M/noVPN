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
    _RU_KEYWORDS = (
        "yandex",
        "vk",
        "mail",
        "rutube",
        "kinopoisk",
        "sber",
        "tinkoff",
        "ozon",
        "wildberries",
        "avito",
        "gosuslugi",
        "2gis",
        "kaspersky",
        "яндекс",
        "вконтакте",
        "почта",
        "рутуб",
        "кинопоиск",
        "сбер",
        "тинькофф",
        "озон",
        "вайлдберриз",
        "авито",
        "госуслуги",
        "каспер",
    )
    _UNINSTALL_ROOTS = (
        ("HKEY_LOCAL_MACHINE", r"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall"),
        ("HKEY_LOCAL_MACHINE", r"SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall"),
        ("HKEY_CURRENT_USER", r"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall"),
    )
    _NOISE_EXECUTABLES = {
        "unins000",
        "uninstall",
        "helper",
        "installer",
        "setup",
        "update",
    }

    def list_candidates(self, extra_paths: list[str] | None = None) -> list[str]:
        entries = self._collect_entries(extra_paths)
        return [path for path, _label in entries]

    def suggest_ru_candidates(self, extra_paths: list[str] | None = None) -> tuple[list[str], list[str]]:
        matched_paths: list[str] = []
        matched_labels: list[str] = []
        for path, label in self._collect_entries(extra_paths):
            haystack = f"{path} {Path(path).stem} {label}".lower()
            if not any(keyword in haystack for keyword in self._RU_KEYWORDS):
                continue
            matched_paths.append(path)
            display = label.strip() or Path(path).stem
            if display and display not in matched_labels:
                matched_labels.append(display)
        return matched_paths, matched_labels

    def _collect_entries(self, extra_paths: list[str] | None = None) -> list[tuple[str, str]]:
        result: list[tuple[str, str]] = []
        seen: set[str] = set()

        def add_entry(path: str, label: str = "") -> None:
            normalized = self.normalize_executable(path)
            if not normalized:
                return
            key = normalized.casefold()
            if key in seen:
                return
            seen.add(key)
            result.append((normalized, label.strip()))

        for raw_path in self._KNOWN_EXECUTABLES:
            add_entry(raw_path)
        for path, label in self._read_registry_entries():
            add_entry(path, label)
        for raw_path in extra_paths or []:
            add_entry(raw_path)
        return result

    def normalize_executable(self, raw_path: str | Path) -> str:
        path = Path(os.path.expandvars(str(raw_path))).expanduser()
        if path.suffix.lower() != ".exe":
            return ""
        if not path.exists():
            return ""
        return str(path.resolve())

    def _read_registry_entries(self) -> list[tuple[str, str]]:
        if os.name != "nt":
            return []
        try:
            import winreg
        except ImportError:
            return []

        result: list[tuple[str, str]] = []
        for root_name, uninstall_path in self._UNINSTALL_ROOTS:
            root = getattr(winreg, root_name)
            try:
                uninstall_key = winreg.OpenKey(root, uninstall_path)
            except OSError:
                continue
            with uninstall_key:
                subkey_count, _value_count, _last_modified = winreg.QueryInfoKey(uninstall_key)
                for index in range(subkey_count):
                    try:
                        subkey_name = winreg.EnumKey(uninstall_key, index)
                    except OSError:
                        continue
                    try:
                        app_key = winreg.OpenKey(uninstall_key, subkey_name)
                    except OSError:
                        continue
                    with app_key:
                        display_name = self._query_registry_value(app_key, "DisplayName")
                        if not display_name:
                            continue
                        display_icon = self._strip_registry_icon_path(self._query_registry_value(app_key, "DisplayIcon"))
                        install_location = self._query_registry_value(app_key, "InstallLocation")
                        display_icon_path = self.normalize_executable(display_icon)
                        if display_icon_path:
                            result.append((display_icon_path, display_name))
                            continue
                        install_candidate = self._guess_executable_from_install_location(install_location, display_name)
                        if install_candidate:
                            result.append((install_candidate, display_name))
        return result

    def _query_registry_value(self, app_key, value_name: str) -> str:
        try:
            import winreg
        except ImportError:
            return ""
        try:
            value, _value_type = winreg.QueryValueEx(app_key, value_name)
        except OSError:
            return ""
        return str(value).strip()

    def _strip_registry_icon_path(self, raw_value: str) -> str:
        value = raw_value.strip().strip("\"")
        if "," not in value:
            return value
        path_part, suffix = value.rsplit(",", 1)
        if suffix.strip().lstrip("-").isdigit():
            return path_part.strip().strip("\"")
        return value

    def _guess_executable_from_install_location(self, raw_install_location: str, display_name: str) -> str:
        location = Path(os.path.expandvars(raw_install_location)).expanduser()
        if not location.is_dir():
            return ""

        candidates = sorted(location.glob("*.exe"))
        if not candidates:
            return ""

        name_hint = self._normalize_name(display_name)
        if name_hint:
            for candidate in candidates:
                stem = self._normalize_name(candidate.stem)
                if stem == name_hint or stem.startswith(name_hint):
                    normalized = self.normalize_executable(candidate)
                    if normalized:
                        return normalized

        for candidate in candidates:
            if self._normalize_name(candidate.stem) in self._NOISE_EXECUTABLES:
                continue
            normalized = self.normalize_executable(candidate)
            if normalized:
                return normalized

        return self.normalize_executable(candidates[0])

    def _normalize_name(self, value: str) -> str:
        cleaned = "".join(ch.lower() for ch in value if ch.isalnum())
        return cleaned[:64]
