from __future__ import annotations

from dataclasses import dataclass

from .models import ClientProfile, ConnectionMode, require_runtime_ready
from .profile_store import ProfileStore
from .runtime_layout import RuntimeLayout
from .windows_tunnel import WindowsSystemTunnelManager


@dataclass(slots=True, frozen=True)
class RuntimePreflightBlocker:
    kind: str
    code: str
    message: str


@dataclass(slots=True)
class RuntimePreflightReport:
    is_ready: bool
    headline: str
    details: list[str]
    blockers: list[RuntimePreflightBlocker]

    def require_ready(self) -> None:
        if not self.is_ready:
            raise RuntimeError(" ".join(self.blocking_messages()))

    def blocking_messages(self) -> list[str]:
        return [blocker.message for blocker in self.blockers]

    def can_fallback_to_local_proxy(self, requested_mode: ConnectionMode) -> bool:
        if requested_mode != ConnectionMode.SYSTEM_TUNNEL:
            return False
        if not self.blockers:
            return False
        return all(blocker.code in _SYSTEM_TUNNEL_FALLBACK_CODES for blocker in self.blockers)

    def fallback_warning(self) -> str:
        reasons = "; ".join(self.blocking_messages())
        return (
            "System tunnel is unavailable for this session. "
            f"{reasons} Switched to local SOCKS/HTTP mode and saved this preference."
        )


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
        profile: ClientProfile | None = None
        if profile_key.strip():
            try:
                profile = self._profile_store.load_by_key(profile_key)
            except Exception as exc:
                code = "profile_invalid" if profile_key.strip() else "profile_missing"
                return RuntimePreflightReport(
                    is_ready=False,
                    headline="Runtime needs attention",
                    details=[str(exc)],
                    blockers=[
                        RuntimePreflightBlocker(
                            kind="profile",
                            code=code,
                            message=str(exc),
                        )
                    ],
                )
        return self.evaluate_profile(profile, connection_mode)

    def evaluate_profile(
        self,
        profile: ClientProfile | None,
        connection_mode: ConnectionMode = ConnectionMode.LOCAL_PROXY,
    ) -> RuntimePreflightReport:
        details: list[str] = []
        blockers: list[RuntimePreflightBlocker] = []

        if profile is None:
            self._add_blocker(
                blockers,
                details,
                kind="profile",
                code="profile_missing",
                message="Activate a code or import a server profile first.",
            )
        else:
            try:
                require_runtime_ready(profile)
            except Exception as exc:
                self._add_blocker(
                    blockers,
                    details,
                    kind="profile",
                    code="profile_invalid",
                    message=str(exc),
                )
            else:
                details.append("Client profile is ready.")

        xray_binary = self._runtime_layout.xray_binary
        obfuscator_binary = self._runtime_layout.obfuscator_binary
        assets_dir = xray_binary.parent

        if xray_binary.is_file():
            details.append(f"Xray found: {xray_binary.name}")
        else:
            self._add_blocker(
                blockers,
                details,
                kind="runtime_binary",
                code="xray_binary_missing",
                message=f"Xray binary not found: {xray_binary}",
            )

        if obfuscator_binary.is_file():
            details.append(f"Obfuscator found: {obfuscator_binary.name}")
        else:
            self._add_blocker(
                blockers,
                details,
                kind="runtime_binary",
                code="obfuscator_binary_missing",
                message=f"Obfuscator binary not found: {obfuscator_binary}",
            )

        for asset_name in ("geoip.dat", "geosite.dat"):
            asset_path = assets_dir / asset_name
            if asset_path.is_file():
                details.append(f"Runtime asset found: {asset_name}")
            else:
                self._add_blocker(
                    blockers,
                    details,
                    kind="runtime_asset",
                    code="runtime_asset_missing",
                    message=f"Runtime asset missing: {asset_path}",
                )

        if connection_mode == ConnectionMode.SYSTEM_TUNNEL:
            if self._windows_tunnel.is_windows():
                if self._windows_tunnel.is_admin():
                    details.append("Administrator context detected for system tunnel mode.")
                else:
                    self._add_blocker(
                        blockers,
                        details,
                        kind="system_tunnel",
                        code="system_tunnel_admin_required",
                        message="System tunnel mode requires launching NoVPN Desktop as Administrator.",
                    )
            else:
                self._add_blocker(
                    blockers,
                    details,
                    kind="system_tunnel",
                    code="system_tunnel_platform_unsupported",
                    message="System tunnel mode is currently available only on Windows.",
                )

            if self._runtime_layout.wintun_dll.is_file():
                details.append(f"Wintun library found: {self._runtime_layout.wintun_dll.name}")
            else:
                self._add_blocker(
                    blockers,
                    details,
                    kind="system_tunnel",
                    code="system_tunnel_wintun_missing",
                    message=f"wintun.dll not found: {self._runtime_layout.wintun_dll}",
                )

            details.append(
                "System-wide mode uses Xray TUN inbound and temporary IPv4 route changes on Windows."
            )
        else:
            details.append("Desktop client will use the local SOCKS/HTTP runtime.")

        return RuntimePreflightReport(
            is_ready=not blockers,
            headline="Runtime ready" if not blockers else "Runtime needs attention",
            details=details,
            blockers=blockers,
        )

    def _add_blocker(
        self,
        blockers: list[RuntimePreflightBlocker],
        details: list[str],
        *,
        kind: str,
        code: str,
        message: str,
    ) -> None:
        blockers.append(RuntimePreflightBlocker(kind=kind, code=code, message=message))
        details.append(message)


_SYSTEM_TUNNEL_FALLBACK_CODES = {
    "system_tunnel_admin_required",
    "system_tunnel_platform_unsupported",
    "system_tunnel_wintun_missing",
}
