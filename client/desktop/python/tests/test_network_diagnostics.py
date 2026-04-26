from __future__ import annotations

import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from novpn_client.models import LocalPorts
from novpn_client.network_diagnostics import NetworkDiagnosticsRunner, _HttpProbeResponse

from tests.helpers import build_profile


class NetworkDiagnosticsRunnerTests(unittest.TestCase):
    def test_run_uses_runtime_status_socks_endpoint(self) -> None:
        runner = _RecordingDiagnosticsRunner()
        profile = build_profile(local=LocalPorts(socks_port=10808, http_port=10809))

        result = runner.run(profile, "127.0.0.1", 23081)

        self.assertIn("Latency", result.summary)
        self.assertTrue(runner.calls)
        self.assertTrue(all(proxy_host == "127.0.0.1" for proxy_host, _, _, _ in runner.calls))
        self.assertTrue(all(proxy_port == 23081 for _, proxy_port, _, _ in runner.calls))


class _RecordingDiagnosticsRunner(NetworkDiagnosticsRunner):
    def __init__(self) -> None:
        super().__init__()
        self.calls: list[tuple[str, int, str, str]] = []

    def _run_stage(self, name: str, operation):
        return operation()

    def _execute_request(
        self,
        proxy_host: str,
        proxy_port: int,
        host: str,
        port: int,
        method: str,
        path: str,
        body: bytes,
    ) -> _HttpProbeResponse:
        self.calls.append((proxy_host, proxy_port, method, path))
        if method == "GET":
            body_bytes = self._DOWNLOAD_BYTES
        elif method == "POST":
            body_bytes = 0
        else:
            body_bytes = 0
        return _HttpProbeResponse(status_code=200, body_bytes=body_bytes)


if __name__ == "__main__":
    unittest.main()
