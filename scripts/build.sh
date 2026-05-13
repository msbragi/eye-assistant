#!/usr/bin/env bash
# GemmaLink — Build & Release script
# Usage: ./scripts/build.sh <win|linux|all> <version>
# Example: ./scripts/build.sh all 1.0.0
#
# Produces:
#   dist/releases/gemmalink-<version>-linux-amd64.tar.gz
#   dist/releases/gemmalink-<version>-windows-amd64.zip

set -euo pipefail

# ─── Validate args ────────────────────────────────────────────────────────────
if [[ $# -ne 2 ]]; then
  echo "Usage: $0 <win|linux|all> <version>"
  echo "  e.g: $0 all 1.0.0"
  exit 1
fi

TARGET="$1"
VERSION="$2"

if [[ "$TARGET" != "win" && "$TARGET" != "linux" && "$TARGET" != "all" ]]; then
  echo "Error: target must be 'win', 'linux' or 'all' (got '$TARGET')"
  exit 1
fi

if [[ -z "$VERSION" ]]; then
  echo "Error: version cannot be empty"
  exit 1
fi

# ─── Paths ────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR/.."
STAGING="$ROOT/dist/staging"
RELEASES="$ROOT/dist/releases"
LDFLAGS="-X main.Version=${VERSION}"

mkdir -p "$RELEASES"

# ─── Helpers ──────────────────────────────────────────────────────────────────

write_config_linux() {
  cat > "$STAGING/config.gl" <<EOF
{
  "sysinfo_refresh_seconds": 5,
  "http_host": "localhost",
  "http_port": "9380",
  "https_port": "9381",
  "llama_local": {
    "enabled": false,
    "endpoint": "http://localhost:9382",
    "model_path": "models/gemma-4-e2b.gguf",
    "mmproj_path": "models/mmproj-gemma-4-e2b.gguf",
    "vision_enabled": false,
    "llama_bin": "bin/linux/llama-server",
    "llama_bin_version": "",
    "context_size": 4096
  },
  "llama_remote": {
    "enabled": false,
    "endpoint": ""
  },
  "model_urls": {
    "e2b": "https://huggingface.co/lmstudio-community/gemma-4-E2B-it-GGUF/resolve/main/gemma-4-E2B-it-Q4_K_M.gguf",
    "e4b": "https://huggingface.co/lmstudio-community/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_K_M.gguf",
    "mmproj_e2b": "",
    "mmproj_e4b": ""
  },
  "upload_dir": "uploads"
}
EOF
}

write_config_windows() {
  cat > "$STAGING/config.gl" <<'EOF'
{
  "sysinfo_refresh_seconds": 5,
  "http_host": "localhost",
  "http_port": "9380",
  "https_port": "9381",
  "llama_local": {
    "enabled": false,
    "endpoint": "http://localhost:9382",
    "model_path": "models/gemma-4-e2b.gguf",
    "mmproj_path": "models/mmproj-gemma-4-e2b.gguf",
    "vision_enabled": false,
    "llama_bin": "bin\\windows\\llama-server.exe",
    "llama_bin_version": "",
    "context_size": 4096
  },
  "llama_remote": {
    "enabled": false,
    "endpoint": ""
  },
  "model_urls": {
    "e2b": "https://huggingface.co/lmstudio-community/gemma-4-E2B-it-GGUF/resolve/main/gemma-4-E2B-it-Q4_K_M.gguf",
    "e4b": "https://huggingface.co/lmstudio-community/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_K_M.gguf",
    "mmproj_e2b": "",
    "mmproj_e4b": ""
  },
  "upload_dir": "uploads"
}
EOF
}

cleanup_staging() {
  # ${STAGING:?} è una protezione: se la variabile è vuota, 
  # lo script abortisce invece di cancellare dalla root.
  echo "- Prepare staging folder ${STAGING}"
  rm -rf "${STAGING:?}"
  mkdir -p "$STAGING"
}

# ─── Build Linux ──────────────────────────────────────────────────────────────
build_linux() {
  cleanup_staging

  echo "▶ Building linux/amd64 v${VERSION}…"

  GOOS=linux GOARCH=amd64 go build \
    -ldflags "$LDFLAGS" \
    -o "$STAGING/gemmalink" \
    "$ROOT"

  write_config_linux
  [[ -f "$ROOT/README.md" ]] && cp "$ROOT/README.md" "$STAGING/README.md"

  local ARCHIVE="$RELEASES/gemmalink-${VERSION}-linux-amd64.tar.gz"
  tar -czf "$ARCHIVE" -C "$STAGING" .
  echo "   ✓ $ARCHIVE"
}

# ─── Build Windows ────────────────────────────────────────────────────────────
build_windows() {
  cleanup_staging

  echo "▶ Building windows/amd64 v${VERSION}…"

  GOOS=windows GOARCH=amd64 go build \
    -ldflags "$LDFLAGS" \
    -o "$STAGING/gemmalink.exe" \
    "$ROOT"

  write_config_windows
  [[ -f "$ROOT/README.md" ]] && cp "$ROOT/README.md" "$STAGING/README.md"

  local ARCHIVE="$RELEASES/gemmalink-${VERSION}-windows-amd64.zip"
  
  # FIX: Rimuovi l'archivio precedente per evitare merge di file obsoleti (es. vecchi config.json)
  rm -f "$ARCHIVE"

  (cd "$STAGING" && zip -rq "$ARCHIVE" .)
  cleanup_staging
  echo "   ✓ $ARCHIVE"
}

# ─── Main ─────────────────────────────────────────────────────────────────────
cd "$ROOT"

case "$TARGET" in
  linux) build_linux ;;
  win)   build_windows ;;
  all)   build_linux; build_windows ;;
esac

echo ""
echo "Release artifacts:"
ls -lh "$RELEASES"/gemmalink-"${VERSION}"-*
