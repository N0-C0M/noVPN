from __future__ import annotations

import ipaddress
import socket
import time
from dataclasses import dataclass

from .models import ClientProfile


@dataclass(slots=True)
class NetworkDiagnosticsResult:
    latency_ms: int
    jitter_ms: int
    download_mbps: float
    upload_mbps: float
    summary: str


class NetworkDiagnosticsRunner:
    def run(self, profile: ClientProfile) -> NetworkDiagnosticsResult:
        host = profile.server.address
        proxy_host = profile.local.socks_listen
        proxy_port = profile.local.socks_port

        latency_samples = []
        for index in range(1, 4):
            started_at = time.perf_counter()
            self._run_stage(
                f"Latency probe #{index}",
                lambda index=index: self._execute_request(
                    proxy_host=proxy_host,
                    proxy_port=proxy_port,
                    host=host,
                    port=self._DIAGNOSTICS_PORT,
                    method="HEAD",
                    path=f"/admin/diag/ping?seq={index}&ts={int(time.time() * 1000)}",
                    body=b"",
                ),
            )
            latency_samples.append(int((time.perf_counter() - started_at) * 1000))

        latency_average = round(sum(latency_samples) / len(latency_samples))
        jitter = max(latency_samples) - min(latency_samples)

        download_started_at = time.perf_counter()
        downloaded_bytes = self._run_stage(
            "Download test",
            lambda: self._execute_request(
                proxy_host=proxy_host,
                proxy_port=proxy_port,
                host=host,
                port=self._DIAGNOSTICS_PORT,
                method="GET",
                path=f"/admin/diag/download?bytes={self._DOWNLOAD_BYTES}",
                body=b"",
            ).body_bytes,
        )
        download_seconds = time.perf_counter() - download_started_at
        download_mbps = self._to_mbps(downloaded_bytes, download_seconds)

        upload_payload = bytes(index % 251 for index in range(self._UPLOAD_BYTES))
        upload_started_at = time.perf_counter()
        self._run_stage(
            "Upload test",
            lambda: self._execute_request(
                proxy_host=proxy_host,
                proxy_port=proxy_port,
                host=host,
                port=self._DIAGNOSTICS_PORT,
                method="POST",
                path="/admin/diag/upload",
                body=upload_payload,
            ),
        )
        upload_seconds = time.perf_counter() - upload_started_at
        upload_mbps = self._to_mbps(len(upload_payload), upload_seconds)

        summary = (
            f"Latency {latency_average} ms | Jitter {jitter} ms\n"
            f"Download {self._format_mbps(download_mbps)} | Upload {self._format_mbps(upload_mbps)}"
        )
        return NetworkDiagnosticsResult(
            latency_ms=latency_average,
            jitter_ms=jitter,
            download_mbps=download_mbps,
            upload_mbps=upload_mbps,
            summary=summary,
        )

    def _execute_request(
        self,
        proxy_host: str,
        proxy_port: int,
        host: str,
        port: int,
        method: str,
        path: str,
        body: bytes,
    ) -> "_HttpProbeResponse":
        with self._open_socks_socket(proxy_host, proxy_port, host, port) as active_socket:
            active_socket.settimeout(self._READ_TIMEOUT_SECONDS)
            request_headers = [
                f"{method} {path} HTTP/1.1",
                f"Host: {host}",
                "Connection: close",
                "Accept: application/json, application/octet-stream",
            ]
            if body:
                request_headers.append("Content-Type: application/octet-stream")
                request_headers.append(f"Content-Length: {len(body)}")
            request_bytes = ("\r\n".join(request_headers) + "\r\n\r\n").encode("ascii")
            active_socket.sendall(request_bytes)
            if body:
                active_socket.sendall(body)

            stream = active_socket.makefile("rb")
            status_line = self._read_ascii_line(stream)
            if not status_line:
                raise RuntimeError(
                    "Сервер диагностики не вернул строку HTTP-статуса. Похоже, туннель не довёл запрос до /admin/diag/ping."
                )
            parts = status_line.split(" ")
            if len(parts) < 2 or not parts[1].isdigit():
                raise RuntimeError(f"Сервер диагностики вернул некорректный HTTP-ответ: {status_line}")
            status_code = int(parts[1])

            content_length = -1
            while True:
                header_line = self._read_ascii_line(stream)
                if not header_line:
                    break
                name, _, value = header_line.partition(":")
                if name.strip().lower() == "content-length":
                    content_length = int(value.strip() or 0)

            if content_length >= 0:
                body_bytes = self._discard_exactly(stream, content_length)
            else:
                body_bytes = self._discard_to_end(stream)

            if status_code < 200 or status_code >= 300:
                raise RuntimeError(f"Сервер диагностики вернул HTTP {status_code}.")
            return _HttpProbeResponse(status_code=status_code, body_bytes=body_bytes)

    def _open_socks_socket(self, proxy_host: str, proxy_port: int, host: str, port: int) -> socket.socket:
        active_socket = socket.create_connection((proxy_host, proxy_port), timeout=self._CONNECT_TIMEOUT_SECONDS)
        active_socket.settimeout(self._HANDSHAKE_TIMEOUT_SECONDS)

        active_socket.sendall(bytes((0x05, 0x01, 0x00)))
        selected_method = self._recv_exact(active_socket, 2)
        if selected_method[0] != 0x05 or selected_method[1] != 0x00:
            raise RuntimeError("Локальный SOCKS-мост вернул некорректный ответ на handshake.")

        destination = self._build_destination_address(host) + port.to_bytes(2, byteorder="big")
        active_socket.sendall(bytes((0x05, 0x01, 0x00)) + destination)

        response_head = self._recv_exact(active_socket, 4)
        if response_head[0] != 0x05 or response_head[1] != 0x00:
            raise RuntimeError(f"Локальный SOCKS отклонил CONNECT с кодом {response_head[1]}.")

        atyp = response_head[3]
        if atyp == 0x01:
            self._recv_exact(active_socket, 4)
        elif atyp == 0x03:
            length = self._recv_exact(active_socket, 1)[0]
            self._recv_exact(active_socket, length)
        elif atyp == 0x04:
            self._recv_exact(active_socket, 16)
        self._recv_exact(active_socket, 2)
        return active_socket

    def _build_destination_address(self, host: str) -> bytes:
        try:
            parsed = ipaddress.ip_address(host)
        except ValueError:
            encoded = host.encode("utf-8")
            return bytes((0x03, len(encoded))) + encoded
        if parsed.version == 4:
            return bytes((0x01,)) + parsed.packed
        return bytes((0x04,)) + parsed.packed

    def _recv_exact(self, active_socket: socket.socket, count: int) -> bytes:
        chunks = bytearray()
        while len(chunks) < count:
            chunk = active_socket.recv(count - len(chunks))
            if not chunk:
                raise RuntimeError("Диагностический поток завершился раньше ожидаемого.")
            chunks.extend(chunk)
        return bytes(chunks)

    def _read_ascii_line(self, stream) -> str:
        buffer = bytearray()
        while True:
            next_byte = stream.read(1)
            if not next_byte:
                break
            if next_byte == b"\n":
                break
            if next_byte != b"\r":
                buffer.extend(next_byte)
        return buffer.decode("ascii", errors="replace")

    def _discard_exactly(self, stream, count: int) -> int:
        remaining = count
        total = 0
        while remaining > 0:
            chunk = stream.read(min(32 * 1024, remaining))
            if not chunk:
                break
            total += len(chunk)
            remaining -= len(chunk)
        return total

    def _discard_to_end(self, stream) -> int:
        total = 0
        while True:
            chunk = stream.read(32 * 1024)
            if not chunk:
                return total
            total += len(chunk)

    def _run_stage(self, name: str, operation):
        try:
            return operation()
        except TimeoutError as exc:
            raise RuntimeError(f"{name}: время ожидания истекло.") from exc
        except socket.timeout as exc:
            raise RuntimeError(f"{name}: время ожидания истекло.") from exc
        except OSError as exc:
            raise RuntimeError(f"{name}: {exc}.") from exc

    def _to_mbps(self, byte_count: int, seconds: float) -> float:
        if seconds <= 0:
            return 0.0
        return (byte_count * 8.0) / seconds / 1_000_000.0

    def _format_mbps(self, value: float) -> str:
        return f"{value:.2f} Mbps"

    _DIAGNOSTICS_PORT = 80
    _DOWNLOAD_BYTES = 1024 * 1024
    _UPLOAD_BYTES = 256 * 1024
    _CONNECT_TIMEOUT_SECONDS = 10
    _HANDSHAKE_TIMEOUT_SECONDS = 15
    _READ_TIMEOUT_SECONDS = 60


@dataclass(slots=True)
class _HttpProbeResponse:
    status_code: int
    body_bytes: int
