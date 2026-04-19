from __future__ import annotations

import json
import re
import uuid
from dataclasses import asdict
from pathlib import Path

from .models import (
    ClientProfile,
    LocalPorts,
    ObfuscationProfile,
    PatternMaskingStrategy,
    ProfileOption,
    ServerProfile,
    TrafficObfuscationStrategy,
    require_runtime_ready,
)


class ProfileStore:
    BUNDLED_PROFILE_KEY = "bundled:default"

    def __init__(
        self,
        bundled_profile_path: Path,
        imported_profiles_dir: Path,
        bootstrap_path: Path,
    ) -> None:
        self._bundled_profile_path = bundled_profile_path
        self._imported_profiles_dir = imported_profiles_dir
        self._bootstrap_path = bootstrap_path
        self._force_server_ip_mode = True

    @property
    def profile_path(self) -> Path:
        return self._bundled_profile_path

    def set_force_server_ip_mode(self, enabled: bool) -> None:
        self._force_server_ip_mode = enabled

    def bootstrap_server_address(self) -> str:
        try:
            payload = json.loads(self._bootstrap_path.read_text(encoding="utf-8"))
        except FileNotFoundError:
            return self._load_bundled_address()
        except json.JSONDecodeError:
            return self._load_bundled_address()
        return str(payload.get("server_address", "")).strip() or self._load_bundled_address()

    def bootstrap_api_base(self) -> str:
        try:
            payload = json.loads(self._bootstrap_path.read_text(encoding="utf-8"))
        except (FileNotFoundError, json.JSONDecodeError):
            return self._load_bundled_api_base()

        api_base = str(payload.get("api_base", "")).strip()
        if api_base:
            return api_base
        return self._load_bundled_api_base()

    def default_profile_key(self) -> str:
        profiles = self.available_profiles()
        return profiles[0].key if profiles else ""

    def available_profiles(self) -> list[ProfileOption]:
        self._imported_profiles_dir.mkdir(parents=True, exist_ok=True)
        options: list[ProfileOption] = []
        try:
            bundled_profile = self.load(self._bundled_profile_path)
        except Exception:
            bundled_profile = None
        if bundled_profile is not None:
            options.append(self._build_option(self.BUNDLED_PROFILE_KEY, bundled_profile, is_imported=False))

        for path in self._list_imported_profile_paths():
            try:
                profile = self.load(path)
            except Exception:
                continue
            options.append(self._build_option(path.name, profile, is_imported=True))
        return options

    def load(self, profile_path: Path | None = None) -> ClientProfile:
        target_path = profile_path or self._bundled_profile_path
        payload = target_path.read_text(encoding="utf-8")
        return self._parse_client_profile_json(payload)

    def load_by_key(self, profile_key: str) -> ClientProfile:
        if not profile_key.strip():
            raise FileNotFoundError("Activate a code or import a profile first.")
        if profile_key == self.BUNDLED_PROFILE_KEY:
            return self.load(self._bundled_profile_path)
        return self.load(self._imported_profiles_dir / profile_key)

    def import_profile_file(self, source_path: Path) -> ProfileOption:
        payload = source_path.read_text(encoding="utf-8")
        return self.import_profile_payload(payload, source_path.name)

    def import_profile_payload(self, payload: str, name_hint: str = "") -> ProfileOption:
        profile = self._parse_imported_payload(payload)
        require_runtime_ready(profile)
        serialized = self._serialize_profile(profile)

        file_name = self._build_imported_file_name(name_hint, profile, serialized)
        output_path = self._imported_profiles_dir / file_name
        self._imported_profiles_dir.mkdir(parents=True, exist_ok=True)
        if not output_path.exists():
            output_path.write_text(serialized, encoding="utf-8")
        return self._build_option(file_name, profile, is_imported=True)

    def delete_profile(self, profile_key: str) -> None:
        target = self._imported_profiles_dir / profile_key
        if target.exists():
            target.unlink()

    def is_imported_profile(self, profile_key: str) -> bool:
        return (self._imported_profiles_dir / profile_key).is_file()

    def _parse_client_profile_json(self, payload: str) -> ClientProfile:
        root = json.loads(payload)
        server = root.get("server", root)
        local = root.get("local", {})
        obfuscation = root.get("obfuscation", {})
        short_id = str(server.get("short_id", "")).strip()
        if not short_id:
            short_ids = server.get("short_ids") or []
            if short_ids:
                short_id = str(short_ids[0]).strip()

        address = self._normalize_server_address(str(server.get("address", "")).strip())
        location_label = str(server.get("location_label", "")).strip() or _SERVER_LOCATION_LABELS.get(address, "")
        seed = str(obfuscation.get("seed", "")).strip() or self._default_seed(short_id)

        return ClientProfile(
            name=str(root.get("name", "")).strip() or "Imported Reality Profile",
            server=ServerProfile(
                address=address,
                port=int(server.get("port", 0) or 0),
                uuid=str(server.get("uuid", "")).strip(),
                flow=str(server.get("flow", "")).strip() or _DEFAULT_FLOW,
                server_name=str(server.get("server_name", "")).strip(),
                fingerprint=str(server.get("fingerprint", "")).strip() or _DEFAULT_FINGERPRINT,
                public_key=str(server.get("public_key", "")).strip(),
                short_id=short_id,
                server_id=str(server.get("server_id", "")).strip(),
                location_label=location_label,
                spider_x=str(server.get("spider_x", "/")).strip() or "/",
                api_base=str(server.get("api_base", "")).strip(),
            ),
            local=LocalPorts(
                socks_listen=str(local.get("socks_listen", "127.0.0.1")).strip() or "127.0.0.1",
                socks_port=int(local.get("socks_port", 10808) or 10808),
                http_listen=str(local.get("http_listen", "127.0.0.1")).strip() or "127.0.0.1",
                http_port=int(local.get("http_port", 10809) or 10809),
            ),
            obfuscation=ObfuscationProfile(
                seed=seed,
                traffic_strategy=TrafficObfuscationStrategy.from_storage(
                    str(obfuscation.get("traffic_strategy", "")).strip() or None
                ),
                pattern_strategy=PatternMaskingStrategy.from_storage(
                    str(obfuscation.get("pattern_strategy", "")).strip() or None
                ),
            ),
        )

    def _parse_imported_payload(self, payload: str) -> ClientProfile:
        trimmed = payload.strip()
        if trimmed.startswith("{"):
            return self._parse_client_profile_json(trimmed)
        return self._parse_server_client_profile_yaml(trimmed)

    def _parse_server_client_profile_yaml(self, payload: str) -> ClientProfile:
        scalars: dict[str, str] = {}
        short_ids: list[str] = []
        active_list: str | None = None

        for raw_line in payload.splitlines():
            line = raw_line.split(" #", 1)[0].rstrip()
            if not line.strip():
                continue

            trimmed = line.strip()
            if trimmed.startswith("- "):
                if active_list == "short_ids":
                    short_ids.append(self._trim_yaml_value(trimmed[2:].strip()))
                continue

            if trimmed.endswith(":"):
                active_list = trimmed[:-1].strip()
                continue

            active_list = None
            if ":" not in trimmed:
                continue

            key, value = trimmed.split(":", 1)
            scalars[key.strip()] = self._trim_yaml_value(value.strip())

        short_id = scalars.get("short_id", "").strip() or (short_ids[0].strip() if short_ids else "")
        address = self._normalize_server_address(scalars.get("address", "").strip())
        location_label = _SERVER_LOCATION_LABELS.get(address, "")

        return ClientProfile(
            name=scalars.get("name", "").strip() or "Imported Reality Profile",
            server=ServerProfile(
                address=address,
                port=int(scalars.get("port", "0") or 0),
                uuid=scalars.get("uuid", "").strip(),
                flow=scalars.get("flow", "").strip() or _DEFAULT_FLOW,
                server_name=scalars.get("server_name", "").strip(),
                fingerprint=scalars.get("fingerprint", "").strip() or _DEFAULT_FINGERPRINT,
                public_key=scalars.get("public_key", "").strip(),
                short_id=short_id,
                server_id=scalars.get("server_id", "").strip(),
                location_label=location_label,
                spider_x=scalars.get("spider_x", "").strip() or "/",
                api_base=scalars.get("api_base", "").strip(),
            ),
            local=LocalPorts(),
            obfuscation=ObfuscationProfile(
                seed=self._default_seed(short_id),
                traffic_strategy=TrafficObfuscationStrategy.BALANCED,
                pattern_strategy=PatternMaskingStrategy.STEADY,
            ),
        )

    def _serialize_profile(self, profile: ClientProfile) -> str:
        payload = {
            "name": profile.name,
            "server": asdict(profile.server),
            "local": asdict(profile.local),
            "obfuscation": {
                "seed": profile.obfuscation.seed,
                "traffic_strategy": profile.obfuscation.traffic_strategy.value,
                "pattern_strategy": profile.obfuscation.pattern_strategy.value,
            },
        }
        return json.dumps(payload, indent=2) + "\n"

    def _build_imported_file_name(self, name_hint: str, profile: ClientProfile, serialized: str) -> str:
        existing_key = self._find_existing_profile_key(serialized)
        if existing_key:
            return existing_key

        parts: list[str] = []
        for candidate in (
            profile.server.server_id,
            name_hint,
            profile.name,
            profile.server.server_name,
            profile.server.location_label,
        ):
            slug = self._slugify(candidate)
            if slug and slug not in parts:
                parts.append(slug)
            if len(parts) >= 2:
                break
        address_slug = self._slugify(profile.server.address.replace(":", "-"))
        if address_slug and address_slug not in parts:
            parts.append(address_slug)

        base_name = "-".join(parts).strip("-")[:72] or f"imported-{uuid.uuid4().hex[:8]}"
        for suffix in ("", *[f"-{index}" for index in range(2, 1_000)]):
            candidate = f"{base_name}{suffix}.profile.json"
            output_path = self._imported_profiles_dir / candidate
            if not output_path.exists():
                return candidate
            if output_path.read_text(encoding="utf-8") == serialized:
                return candidate
        return f"imported-{uuid.uuid4().hex[:8]}.profile.json"

    def _slugify(self, value: str) -> str:
        normalized = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
        return normalized[:48]

    def _trim_yaml_value(self, value: str) -> str:
        trimmed = value.strip()
        if trimmed.startswith(("\"", "'")) and trimmed.endswith(("\"", "'")) and len(trimmed) >= 2:
            return trimmed[1:-1]
        return trimmed

    def _normalize_server_address(self, value: str) -> str:
        address = value.strip()
        if not address or self._is_numeric_address(address):
            return address

        if self._force_server_ip_mode:
            fallback = self.bootstrap_server_address()
            if self._is_numeric_address(fallback):
                return fallback

        lowered = address.lower()
        if lowered == _PENDING_ROOT_DOMAIN or lowered.endswith(f".{_PENDING_ROOT_DOMAIN}"):
            return self.bootstrap_server_address() or address
        return address

    def _is_numeric_address(self, value: str) -> bool:
        if ":" in value:
            return True
        return bool(_IPV4_RE.match(value))

    def _load_bundled_address(self) -> str:
        try:
            payload = json.loads(self._bundled_profile_path.read_text(encoding="utf-8"))
        except (FileNotFoundError, json.JSONDecodeError):
            return ""
        server = payload.get("server", payload)
        return str(server.get("address", "")).strip()

    def _load_bundled_api_base(self) -> str:
        try:
            payload = json.loads(self._bundled_profile_path.read_text(encoding="utf-8"))
        except (FileNotFoundError, json.JSONDecodeError):
            return ""
        server = payload.get("server", payload)
        return str(server.get("api_base", "")).strip()

    def _list_imported_profile_paths(self) -> list[Path]:
        seen: dict[str, Path] = {}
        for pattern in ("*.profile.json", "profile.*.json"):
            for path in self._imported_profiles_dir.glob(pattern):
                seen[path.name] = path
        return [seen[name] for name in sorted(seen, key=str.lower)]

    def _find_existing_profile_key(self, serialized: str) -> str:
        for path in self._list_imported_profile_paths():
            try:
                if path.read_text(encoding="utf-8") == serialized:
                    return path.name
            except OSError:
                continue
        return ""

    def _build_option(self, key: str, profile: ClientProfile, is_imported: bool) -> ProfileOption:
        return ProfileOption(
            key=key,
            name=profile.name,
            address=f"{profile.server.address}:{profile.server.port}",
            server_name=profile.server.server_name,
            location_label=profile.server.location_label,
            is_imported=is_imported,
            server_id=profile.server.server_id,
        )

    def _default_seed(self, short_id: str) -> str:
        base = short_id.strip() or uuid.uuid4().hex
        return f"novpn-seed-{base}"


_DEFAULT_FLOW = "xtls-rprx-vision"
_DEFAULT_FINGERPRINT = "chrome"
_PENDING_ROOT_DOMAIN = "xower.eu.org"
_IPV4_RE = re.compile(r"^\d{1,3}(\.\d{1,3}){3}$")
_SERVER_LOCATION_LABELS = {
    "2.26.85.47": "Finland",
    "87.121.105.190": "Switzerland (fast)",
}
