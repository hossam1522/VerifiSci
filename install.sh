#!/bin/sh
set -e

# VerifiSci Installer Script
# Automatically detects OS and Architecture, downloads the latest release, and installs it.

REPO="hossam1522/VerifiSci"
GITHUB_URL="https://github.com/${REPO}"

# Helper print functions
info() {
  printf "\033[34m[INFO]\033[0m %s\n" "$1"
}

success() {
  printf "\033[32m[SUCCESS]\033[0m %s\n" "$1"
}

error() {
  printf "\033[31m[ERROR]\033[0m %s\n" "$1" >&2
  exit 1
}

# 1. Detect OS
detect_os() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$OS" in
    linux*)
      OS="linux"
      ;;
    darwin*)
      OS="darwin"
      ;;
    msys*|mingw*|cygwin*)
      OS="windows"
      ;;
    *)
      error "Unsupported Operating System: $(uname -s)"
      ;;
  esac
  echo "$OS"
}

# 2. Detect Architecture
detect_arch() {
  ARCH="$(uname -m | tr '[:upper:]' '[:lower:]')"
  case "$ARCH" in
    x86_64|amd64)
      ARCH="amd64"
      ;;
    arm64|aarch64|armv8*)
      ARCH="arm64"
      ;;
    *)
      error "Unsupported CPU architecture: $(uname -m)"
      ;;
  esac
  echo "$ARCH"
}

# 3. Detect latest release version if not specified
get_latest_version() {
  if [ -n "$VERSION" ]; then
    echo "$VERSION"
    return
  fi

  # Attempt 1: Via GitHub HTTP redirect (does not count towards GitHub API rate limits)
  TAG=$(curl -sSLI -o /dev/null -w '%{url_effective}' "${GITHUB_URL}/releases/latest" 2>/dev/null | grep -oE '[^/]+$' || true)

  # Attempt 2: Fallback to GitHub API
  if [ -z "$TAG" ] || [ "$TAG" = "latest" ] || [ "$TAG" = "releases" ]; then
    TAG=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
  fi

  if [ -z "$TAG" ]; then
    error "Could not determine latest release version. Please specify VERSION=vX.Y.Z"
  fi

  echo "$TAG"
}

main() {
  OS=$(detect_os)
  ARCH=$(detect_arch)
  VERSION=$(get_latest_version)

  info "Installing VerifiSci ${VERSION} for ${OS}/${ARCH}..."

  # Determine archive extension
  EXT="tar.gz"
  if [ "$OS" = "windows" ]; then
    EXT="zip"
  fi

  ARCHIVE_NAME="verifisci-${VERSION}-${OS}-${ARCH}.${EXT}"
  DOWNLOAD_URL="${GITHUB_URL}/releases/download/${VERSION}/${ARCHIVE_NAME}"

  # Create temporary directory
  TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'verifisci')
  cleanup() {
    rm -rf "$TMP_DIR"
  }
  trap cleanup EXIT INT TERM

  info "Downloading ${DOWNLOAD_URL}..."
  if ! curl -fSL --progress-bar "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
    # Fallback to unversioned archive name if versioned download failed
    ALT_ARCHIVE="verifisci-${OS}-${ARCH}.${EXT}"
    ALT_URL="${GITHUB_URL}/releases/download/${VERSION}/${ALT_ARCHIVE}"
    if ! curl -fSL --progress-bar "$ALT_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
      error "Failed to download VerifiSci archive from ${DOWNLOAD_URL}"
    fi
  fi

  # Extract archive
  info "Extracting binary..."
  if [ "$EXT" = "zip" ]; then
    unzip -q -o "${TMP_DIR}/${ARCHIVE_NAME}" -d "$TMP_DIR"
  else
    tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"
  fi

  # Find the extracted binary (handles verifisci, verifisci-linux-amd64, verifisci.exe, etc.)
  BIN_SRC=""
  for candidate in \
    "${TMP_DIR}/verifisci" \
    "${TMP_DIR}/verifisci-${OS}-${ARCH}" \
    "${TMP_DIR}/verifisci-${OS}-${ARCH}.exe" \
    "${TMP_DIR}/verifisci.exe"; do
    if [ -f "$candidate" ]; then
      BIN_SRC="$candidate"
      break
    fi
  done

  if [ -z "$BIN_SRC" ]; then
    # Search for any executable binary inside TMP_DIR
    BIN_SRC=$(find "$TMP_DIR" -type f -name "verifisci*" ! -name "*.tar.gz" ! -name "*.zip" | head -n 1)
  fi

  if [ -z "$BIN_SRC" ] || [ ! -f "$BIN_SRC" ]; then
    error "Could not find extracted verifisci binary."
  fi

  chmod +x "$BIN_SRC"

  # Determine installation directory
  TARGET_DIR="${INSTALL_DIR:-/usr/local/bin}"
  BIN_NAME="verifisci"
  if [ "$OS" = "windows" ]; then
    BIN_NAME="verifisci.exe"
  fi
  TARGET_PATH="${TARGET_DIR}/${BIN_NAME}"

  mkdir -p "$TARGET_DIR" 2>/dev/null || true

  info "Installing to ${TARGET_PATH}..."

  if [ -w "$TARGET_DIR" ]; then
    mv "$BIN_SRC" "$TARGET_PATH"
  else
    if command -v sudo >/dev/null 2>&1 && [ -t 0 ]; then
      sudo mv "$BIN_SRC" "$TARGET_PATH"
    else
      # If cannot write to /usr/local/bin and cannot sudo, try ~/.local/bin
      USER_BIN="$HOME/.local/bin"
      mkdir -p "$USER_BIN"
      TARGET_PATH="${USER_BIN}/${BIN_NAME}"
      mv "$BIN_SRC" "$TARGET_PATH"
      info "Installed to ${TARGET_PATH} (since ${TARGET_DIR} is not user-writable)."
      case ":$PATH:" in
        *":$USER_BIN:"*) ;;
        *)
          printf "\033[33m[WARNING]\033[0m %s is not in your PATH. Add it with: export PATH=\"\$HOME/.local/bin:\$PATH\"\n" "$USER_BIN"
          ;;
      esac
    fi
  fi

  chmod +x "$TARGET_PATH"

  success "VerifiSci was successfully installed!"
  if command -v "$BIN_NAME" >/dev/null 2>&1; then
    "$BIN_NAME" version || true
  else
    "$TARGET_PATH" version || true
  fi
}

main "$@"
