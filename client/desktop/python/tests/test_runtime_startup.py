from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from novpn_client.models import ClientState, ConnectionMode, DesktopSettings
from novpn_client.runtime_preflight import RuntimePreflightBlocker, RuntimePreflightReport
from novpn_client.runtime_startup import prepare_runtime_start
from novpn_client.state_store import ClientStateStore


class PrepareRuntimeStartTests(unittest.TestCase):
    def test_fallback_switches_to_local_proxy_and_persists_state(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            state_store = ClientStateStore(Path(tmp_dir) / "client_state.json")
            state_store.save(ClientState(connection_mode=ConnectionMode.SYSTEM_TUNNEL))
            checker = _FakePreflightChecker(
                system_tunnel_report=RuntimePreflightReport(
                    is_ready=False,
                    headline="Runtime needs attention",
                    details=["System tunnel mode requires launching NoVPN Desktop as Administrator."],
                    blockers=[
                        RuntimePreflightBlocker(
                            kind="system_tunnel",
                            code="system_tunnel_admin_required",
                            message="System tunnel mode requires launching NoVPN Desktop as Administrator.",
                        )
                    ],
                ),
                local_proxy_report=RuntimePreflightReport(
                    is_ready=True,
                    headline="Runtime ready",
                    details=["Desktop client will use the local SOCKS/HTTP runtime."],
                    blockers=[],
                ),
            )

            settings = DesktopSettings(
                bypass_ru=True,
                app_routing_mode=ClientState().app_routing_mode,
                selected_apps=[],
                traffic_strategy=ClientState().traffic_strategy,
                pattern_strategy=ClientState().pattern_strategy,
                connection_mode=ConnectionMode.SYSTEM_TUNNEL,
                device_id="device-id",
                output_path=Path(tmp_dir) / "config.json",
            )

            prepared = prepare_runtime_start(
                checker,
                settings,
                profile_key="bundled:default",
                persist_connection_mode=lambda mode: state_store.update(
                    state_store.load(),
                    connection_mode=mode,
                ),
            )

            saved_state = state_store.load()

        self.assertEqual(prepared.settings.connection_mode, ConnectionMode.LOCAL_PROXY)
        self.assertEqual(saved_state.connection_mode, ConnectionMode.LOCAL_PROXY)
        self.assertIn("Switched to local SOCKS/HTTP mode", prepared.fallback_warning)


class _FakePreflightChecker:
    def __init__(
        self,
        *,
        system_tunnel_report: RuntimePreflightReport,
        local_proxy_report: RuntimePreflightReport,
    ) -> None:
        self._reports = {
            ConnectionMode.SYSTEM_TUNNEL: system_tunnel_report,
            ConnectionMode.LOCAL_PROXY: local_proxy_report,
        }

    def evaluate(self, profile_key: str, connection_mode: ConnectionMode) -> RuntimePreflightReport:
        return self._reports[connection_mode]

    def evaluate_profile(self, profile, connection_mode: ConnectionMode) -> RuntimePreflightReport:
        return self._reports[connection_mode]


if __name__ == "__main__":
    unittest.main()
