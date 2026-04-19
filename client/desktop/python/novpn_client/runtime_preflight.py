from __future__ import annotations

from dataclasses import dataclass

from .models import ConnectionMode, require_runtime_ready
from .profile_store import ProfileStore
from .runtime_layout import RuntimeLayout
from .windows_tunnel import WindowsSystemTunnelManager


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
        self._windows_tunnel = WindowsSystemTunnelManager()

    def evaluate(
        self,
        profile_key: str,
        connection_mode: ConnectionMode = ConnectionMode.LOCAL_PROXY,
    ) -> RuntimePreflightReport:
        ready = True
        details: list[str] = []

        if not profile_key.strip():
            ready = False
            details.append("Activate a code or import a server profile first.")
        else:
            try:
                require_runtime_ready(self._profile_store.load_by_key(profile_key))
            except Exception as exc:
                ready = False
                details.append(str(exc))
            else:
                details.append("Client profile is ready.")

        xray_binary = self._runtime_layout.xray_binary
        obfuscator_binary = self._runtime_layout.obfuscator_binary
        assets_dir = xray_binary.parent

        if xray_binary.is_file():
            details.append(f"Xray found: {xray_binary.name}")
        else:
            ready = False
            details.append(f"Xray not found: {xray_binary}")

        if obfuscator_binary.is_file():
            details.append(f"Obfuscator found: {obfuscator_binary.name}")
        else:
            ready = False
            details.append(f"Obfuscator not found: {obfuscator_binary}")

        for asset_name in ("geoip.dat", "geosite.dat"):
            asset_path = assets_dir / asset_name
            if asset_path.is_file():
                details.append(f"Runtime asset found: {asset_name}")
            else:
                ready = False
                details.append(f"Runtime asset missing: {asset_path}")

        if connection_mode == ConnectionMode.SYSTEM_TUNNEL:
            if self._windows_tunnel.is_windows():
                if self._windows_tunnel.is_admin():
                    details.append("Administrator context detected for system tunnel mode.")
                else:
                    ready = False
                    details.append("System tunnel mode requires launching NoVPN Desktop as Administrator.")
            else:
                ready = False
                details.append("System tunnel mode is currently implemented only on Windows.")

            if self._runtime_layout.wintun_dll.is_file():
                details.append(f"Wintun library found: {self._runtime_layout.wintun_dll.name}")
            else:
                ready = False
                details.append(f"wintun.dll not found: {self._runtime_layout.wintun_dll}")

            details.append(
                "System-wide mode uses Xray TUN inbound and temporary IPv4 route changes on Windows."
            )
        else:
            details.append("Desktop client will use the local SOCKS/HTTP runtime.")

        return RuntimePreflightReport(
            is_ready=ready,
            headline="Environment ready" if ready else "Environment needs attention",
            details=details,
        )
