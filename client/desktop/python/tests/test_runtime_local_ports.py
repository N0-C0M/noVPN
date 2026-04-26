from __future__ import annotations

import socket
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from novpn_client.models import LocalPorts
from novpn_client.runtime_local_ports import RuntimeLocalPortResolver


class RuntimeLocalPortResolverTests(unittest.TestCase):
    def setUp(self) -> None:
        self.resolver = RuntimeLocalPortResolver()

    def test_uses_requested_ports_when_available(self) -> None:
        requested = LocalPorts(
            socks_listen="127.0.0.1",
            socks_port=self._free_port(),
            http_listen="127.0.0.1",
            http_port=self._free_port(),
        )

        resolved = self.resolver.resolve(requested)

        self.assertEqual(resolved.local_ports, requested)
        self.assertEqual(resolved.warnings, [])

    def test_falls_back_to_session_ports_when_requested_ports_are_busy(self) -> None:
        with self._bound_listener() as busy_socks, self._bound_listener() as busy_http:
            requested = LocalPorts(
                socks_listen="127.0.0.1",
                socks_port=busy_socks.getsockname()[1],
                http_listen="127.0.0.1",
                http_port=busy_http.getsockname()[1],
            )

            resolved = self.resolver.resolve(requested)

        self.assertNotEqual(resolved.local_ports.socks_port, requested.socks_port)
        self.assertNotEqual(resolved.local_ports.http_port, requested.http_port)
        self.assertEqual(resolved.local_ports.socks_listen, "127.0.0.1")
        self.assertEqual(resolved.local_ports.http_listen, "127.0.0.1")
        self.assertTrue(resolved.warnings)

    def _free_port(self) -> int:
        with self._bound_listener() as listener:
            return listener.getsockname()[1]

    def _bound_listener(self) -> socket.socket:
        listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        listener.bind(("127.0.0.1", 0))
        return listener


if __name__ == "__main__":
    unittest.main()
