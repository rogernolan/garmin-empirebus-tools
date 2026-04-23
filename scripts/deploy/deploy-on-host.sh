#!/usr/bin/env bash
set -euo pipefail

ARTIFACT=""
RELEASE=""
INSTALL_ROOT="/opt/xtura"
CONFIG_PATH="/var/lib/xtura/config.yaml"
SERVICE_NAME="empirebusd"
SERVICE_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --artifact)
      ARTIFACT="$2"
      shift 2
      ;;
    --release)
      RELEASE="$2"
      shift 2
      ;;
    --install-root)
      INSTALL_ROOT="$2"
      shift 2
      ;;
    --config)
      CONFIG_PATH="$2"
      shift 2
      ;;
    --service)
      SERVICE_NAME="$2"
      shift 2
      ;;
    --service-file)
      SERVICE_FILE="$2"
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "${ARTIFACT}" || -z "${RELEASE}" ]]; then
  echo "--artifact and --release are required" >&2
  exit 1
fi

if [[ ! -f "${ARTIFACT}" ]]; then
  echo "artifact not found: ${ARTIFACT}" >&2
  exit 1
fi

CONFIG_DIR="$(dirname "${CONFIG_PATH}")"
RELEASES_DIR="${INSTALL_ROOT}/releases"
RELEASE_DIR="${RELEASES_DIR}/${RELEASE}"
CURRENT_LINK="${INSTALL_ROOT}/current"
SYSTEMD_DIR="/etc/systemd/system"
SERVICE_PATH="${SYSTEMD_DIR}/${SERVICE_NAME}.service"

mkdir -p "${RELEASES_DIR}" "${CONFIG_DIR}"

if [[ ! -f "${CONFIG_PATH}" ]]; then
  echo "config file not found: ${CONFIG_PATH}" >&2
  echo "create it from config.example.yaml before deploying" >&2
  exit 1
fi

rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"
tar -C "${RELEASE_DIR}" -xzf "${ARTIFACT}"

if [[ -n "${SERVICE_FILE}" ]]; then
  install -m 0644 "${SERVICE_FILE}" "${SERVICE_PATH}"
fi

ln -sfn "${RELEASE_DIR}" "${CURRENT_LINK}"
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}.service"
systemctl restart "${SERVICE_NAME}.service"
systemctl --no-pager --full status "${SERVICE_NAME}.service"
