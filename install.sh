#!/usr/bin/env sh
set -eu

REPO="joshuadavidthomas/vibeusage"
INSTALL_DIR="${VIBEUSAGE_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VIBEUSAGE_VERSION:-latest}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

download() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
    return
  fi
  echo "error: install requires curl or wget" >&2
  exit 1
}

sha256_file() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  echo "error: install requires sha256sum or shasum" >&2
  exit 1
}

normalize_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "error: unsupported OS $(uname -s)" >&2
      exit 1
      ;;
  esac
}

normalize_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "error: unsupported architecture $(uname -m)" >&2
      exit 1
      ;;
  esac
}

need_cmd tar
need_cmd awk
need_cmd mktemp

OS="$(normalize_os)"
ARCH="$(normalize_arch)"
ASSET="vibeusage_${OS}_${ARCH}.tar.gz"
CHECKSUMS="checksums.txt"

if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
  CHECKSUMS_URL="https://github.com/${REPO}/releases/latest/download/${CHECKSUMS}"
else
  case "$VERSION" in
    v*) TAG="$VERSION" ;;
    *) TAG="v$VERSION" ;;
  esac
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
  CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/${CHECKSUMS}"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

ARCHIVE_PATH="$TMP_DIR/$ASSET"
CHECKSUMS_PATH="$TMP_DIR/$CHECKSUMS"

echo "Downloading ${ASSET}..."
download "$DOWNLOAD_URL" "$ARCHIVE_PATH"
download "$CHECKSUMS_URL" "$CHECKSUMS_PATH"

EXPECTED_SUM="$(awk -v name="$ASSET" '$2 == name || $2 == "*"name { print $1; exit }' "$CHECKSUMS_PATH")"
if [ -z "$EXPECTED_SUM" ]; then
  echo "error: could not find checksum entry for ${ASSET}" >&2
  exit 1
fi

ACTUAL_SUM="$(sha256_file "$ARCHIVE_PATH")"
if [ "$EXPECTED_SUM" != "$ACTUAL_SUM" ]; then
  echo "error: checksum mismatch for ${ASSET}" >&2
  echo "expected: $EXPECTED_SUM" >&2
  echo "actual:   $ACTUAL_SUM" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

BIN_PATH="$TMP_DIR/vibeusage"
if [ ! -f "$BIN_PATH" ]; then
  echo "error: release archive did not contain vibeusage binary" >&2
  exit 1
fi

if command -v install >/dev/null 2>&1; then
  install -m 0755 "$BIN_PATH" "$INSTALL_DIR/vibeusage"
else
  cp "$BIN_PATH" "$INSTALL_DIR/vibeusage"
  chmod 0755 "$INSTALL_DIR/vibeusage"
fi

printf '%s\n' 'install-script' > "$INSTALL_DIR/.vibeusage-managed-by"

echo "Installed vibeusage to $INSTALL_DIR/vibeusage"
echo "Run: vibeusage --version"
