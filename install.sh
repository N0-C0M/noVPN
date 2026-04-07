#!/usr/bin/env bash

set -euo pipefail

SERVER_PUBLIC_HOST="${SERVER_PUBLIC_HOST:-2.26.85.47}"
SERVER_PUBLIC_PORT="${SERVER_PUBLIC_PORT:-8443}"
REALITY_TARGET="${REALITY_TARGET:-www.cloudflare.com:443}"
REALITY_SERVER_NAME="${REALITY_SERVER_NAME:-www.cloudflare.com}"
APP_DIR="${APP_DIR:-/opt/noVPN}"
CONFIG_DIR="${CONFIG_DIR:-/etc/gateway}"
CONFIG_PATH="${CONFIG_PATH:-$CONFIG_DIR/config.yaml}"
SERVICE_PATH="/etc/systemd/system/gateway.service"
CLIENT_PROFILE_PATH="/var/lib/novpn/reality/client-profile.yaml"
GO_VERSION="${GO_VERSION:-1.22.12}"
MIN_GO_VERSION="1.22.0"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log() {
  printf '[install] %s\n' "$*"
}

fail() {
  printf '[install] error: %s\n' "$*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

version_ge() {
  [[ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "run this script as root: sudo bash install.sh"
  fi
}

detect_go_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      printf 'amd64\n'
      ;;
    aarch64|arm64)
      printf 'arm64\n'
      ;;
    *)
      fail "unsupported CPU architecture: $(uname -m)"
      ;;
  esac
}

install_base_packages() {
  if ! command_exists apt-get; then
    fail "this installer currently supports Debian/Ubuntu only"
  fi

  log "installing base packages"
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    curl \
    git \
    build-essential \
    python3 \
    tar \
    sed
}

ensure_go() {
  if command_exists go; then
    local current_version
    current_version="$(go env GOVERSION 2>/dev/null || true)"
    current_version="${current_version#go}"
    if [[ -n "${current_version}" ]] && version_ge "${current_version}" "${MIN_GO_VERSION}"; then
      log "using existing Go ${current_version}"
      return
    fi
  fi

  local go_arch archive_url archive_path
  go_arch="$(detect_go_arch)"
  archive_url="https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz"
  archive_path="/tmp/go${GO_VERSION}.linux-${go_arch}.tar.gz"

  log "installing Go ${GO_VERSION}"
  curl -fsSL "${archive_url}" -o "${archive_path}"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "${archive_path}"
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
  export PATH="/usr/local/go/bin:${PATH}"
}

prepare_directories() {
  log "preparing directories"
  install -d -m 0755 "${APP_DIR}" "${CONFIG_DIR}"
}

prepare_config() {
  log "preparing ${CONFIG_PATH}"
  if [[ ! -f "${CONFIG_PATH}" ]]; then
    install -m 0644 "${SCRIPT_DIR}/deploy/config.example.yaml" "${CONFIG_PATH}"
  fi

  sed -E -i \
    -e "s|^([[:space:]]*)public_host:[[:space:]]*.*$|\1public_host: ${SERVER_PUBLIC_HOST}|" \
    -e "s|^([[:space:]]*)public_port:[[:space:]]*.*$|\1public_port: ${SERVER_PUBLIC_PORT}|" \
    -e "s|^([[:space:]]*)target:[[:space:]]*.*$|\1target: ${REALITY_TARGET}|" \
    -e "s|^([[:space:]]*)listen_addr:[[:space:]]*0\.0\.0\.0:.*$|\1listen_addr: 0.0.0.0:${SERVER_PUBLIC_PORT}|" \
    "${CONFIG_PATH}"

  python3 - <<'PY' "${CONFIG_PATH}" "${REALITY_SERVER_NAME}"
import sys
from pathlib import Path

config_path = Path(sys.argv[1])
server_name = sys.argv[2]
lines = config_path.read_text(encoding="utf-8").splitlines()
result = []
inside_server_names = False
replaced = False

for line in lines:
    stripped = line.strip()
    if stripped == "server_names:":
        inside_server_names = True
        result.append(line)
        continue
    if inside_server_names and stripped.startswith("- "):
        if not replaced:
            indent = line[: len(line) - len(line.lstrip())]
            result.append(f"{indent}- {server_name}")
            replaced = True
        continue
    if inside_server_names and stripped and not stripped.startswith("- "):
        if not replaced:
            result.append("      - " + server_name)
            replaced = True
        inside_server_names = False
    result.append(line)

if inside_server_names and not replaced:
    result.append("      - " + server_name)

config_path.write_text("\n".join(result) + "\n", encoding="utf-8")
PY
}

build_binaries() {
  log "building gateway"
  (
    cd "${SCRIPT_DIR}"
    go build -o /usr/local/bin/gateway ./cmd/gateway
    go build -o /usr/local/bin/reality-bootstrap ./cmd/reality-bootstrap
  )
  chmod 0755 /usr/local/bin/gateway /usr/local/bin/reality-bootstrap
}

bootstrap_reality() {
  log "bootstrapping Xray Reality"
  /usr/local/bin/reality-bootstrap -config "${CONFIG_PATH}"
}

install_gateway_service() {
  log "installing gateway.service"
  cat > "${SERVICE_PATH}" <<EOF
[Unit]
Description=Transport Gateway for VPN core
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${APP_DIR}
ExecStart=/usr/local/bin/gateway -config ${CONFIG_PATH}
Restart=always
RestartSec=2
LimitNOFILE=200000

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now gateway
}

open_firewall() {
  if command_exists ufw; then
    log "opening ${SERVER_PUBLIC_PORT}/tcp in ufw"
    ufw allow "${SERVER_PUBLIC_PORT}/tcp" >/dev/null || true
  fi
}

print_summary() {
  log "installation completed"
  printf '\n'
  printf 'Server address: %s\n' "${SERVER_PUBLIC_HOST}"
  printf 'Gateway config: %s\n' "${CONFIG_PATH}"
  printf 'Client profile: %s\n' "${CLIENT_PROFILE_PATH}"
  printf '\n'
  systemctl --no-pager --full status xray gateway || true
  printf '\n'
  if [[ -f "${CLIENT_PROFILE_PATH}" ]]; then
    log "generated client profile"
    cat "${CLIENT_PROFILE_PATH}"
  fi
}

main() {
  require_root
  install_base_packages
  ensure_go
  prepare_directories
  prepare_config
  build_binaries
  bootstrap_reality
  install_gateway_service
  open_firewall
  print_summary
}

main "$@"
