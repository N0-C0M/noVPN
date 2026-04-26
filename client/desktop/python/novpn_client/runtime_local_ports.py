from __future__ import annotations

import ipaddress
import socket
from dataclasses import dataclass

from .models import LocalPorts


@dataclass(slots=True)
class ResolvedLocalPorts:
    local_ports: LocalPorts
    warnings: list[str]


class RuntimeLocalPortResolver:
    def resolve(self, requested: LocalPorts) -> ResolvedLocalPorts:
        preferred = LocalPorts(
            socks_listen=self._normalize_requested_host(requested.socks_listen),
            socks_port=int(requested.socks_port),
            http_listen=self._normalize_requested_host(requested.http_listen),
            http_port=int(requested.http_port),
        )

        if self._preferred_ports_available(preferred):
            return ResolvedLocalPorts(local_ports=preferred, warnings=[])

        fallback = LocalPorts(
            socks_listen=self._normalize_loopback_host(preferred.socks_listen),
            socks_port=0,
            http_listen=self._normalize_loopback_host(preferred.http_listen),
            http_port=0,
        )
        reserved_sockets = [
            self._reserve_ephemeral_listener(fallback.socks_listen),
            self._reserve_ephemeral_listener(fallback.http_listen),
        ]
        try:
            effective = LocalPorts(
                socks_listen=fallback.socks_listen,
                socks_port=reserved_sockets[0].getsockname()[1],
                http_listen=fallback.http_listen,
                http_port=reserved_sockets[1].getsockname()[1],
            )
        finally:
            for reserved_socket in reserved_sockets:
                reserved_socket.close()

        warning = (
            "Configured local runtime ports are unavailable. "
            f"Using session-only loopback ports SOCKS {effective.socks_listen}:{effective.socks_port} "
            f"and HTTP {effective.http_listen}:{effective.http_port}."
        )
        return ResolvedLocalPorts(local_ports=effective, warnings=[warning])

    def _preferred_ports_available(self, local_ports: LocalPorts) -> bool:
        if not self._is_valid_tcp_port(local_ports.socks_port):
            return False
        if not self._is_valid_tcp_port(local_ports.http_port):
            return False
        if (
            local_ports.socks_listen == local_ports.http_listen
            and local_ports.socks_port == local_ports.http_port
        ):
            return False
        return self._can_bind(local_ports.socks_listen, local_ports.socks_port) and self._can_bind(
            local_ports.http_listen,
            local_ports.http_port,
        )

    def _can_bind(self, host: str, port: int) -> bool:
        try:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as candidate:
                candidate.bind((host, port))
        except OSError:
            return False
        return True

    def _reserve_ephemeral_listener(self, host: str) -> socket.socket:
        candidate = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        candidate.bind((host, 0))
        return candidate

    def _normalize_requested_host(self, host: str) -> str:
        candidate = host.strip() or "127.0.0.1"
        if candidate.lower() == "localhost":
            return candidate
        try:
            ipaddress.ip_address(candidate)
        except ValueError:
            return candidate
        return candidate

    def _normalize_loopback_host(self, host: str) -> str:
        candidate = host.strip() or "127.0.0.1"
        if candidate.lower() == "localhost":
            return "127.0.0.1"
        try:
            parsed = ipaddress.ip_address(candidate)
        except ValueError:
            return "127.0.0.1"
        if parsed.version != 4 or not parsed.is_loopback:
            return "127.0.0.1"
        return candidate

    def _is_valid_tcp_port(self, port: int) -> bool:
        return 1 <= int(port) <= 65535
