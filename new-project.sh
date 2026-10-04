#!/usr/bin/env bash
# ==============================================================================
# ⚡ TGo Booster — Instant New Project Creator Script (Linux / macOS / WSL)
# ==============================================================================
set -e

echo "=================================================================="
echo "⚡ TGo Booster — Instant New Project Creator"
echo "=================================================================="

PROJECT_NAME="$1"
PORT="$2"
DB="${3:-sqlite}"

if [ -z "$PROJECT_NAME" ]; then
    read -p "Masukkan nama proyek baru (contoh: cms-portal, tokoku): " PROJECT_NAME
fi

PROJECT_NAME=$(echo "$PROJECT_NAME" | xargs)
if [ -z "$PROJECT_NAME" ]; then
    echo "❌ Nama proyek tidak boleh kosong!"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BOOSTER_BIN="$SCRIPT_DIR/booster"

if [ ! -f "$BOOSTER_BIN" ]; then
    echo "⚙️ Mengompilasi booster CLI..."
    go build -o "$BOOSTER_BIN" "$SCRIPT_DIR/cmd/booster"
fi

ARGS=("new" "$PROJECT_NAME" "--db" "$DB")
if [ -n "$PORT" ]; then
    ARGS+=("--port" "$PORT")
fi

"$BOOSTER_BIN" "${ARGS[@]}"
