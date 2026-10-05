#!/bin/sh
set -eu
DATA_DIR="${DATA_DIR:-/var/data}"
BACKUP_DIR="${BACKUP_DIR:-$DATA_DIR/backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE="$BACKUP_DIR/worktic_pre_v26_${STAMP}.tar.gz"
mkdir -p "$BACKUP_DIR"
if [ ! -f "$DATA_DIR/worktic.db" ]; then
  echo "ERROR: no existe $DATA_DIR/worktic.db" >&2
  exit 1
fi
items="worktic.db"
[ -d "$DATA_DIR/wa_sessions" ] && items="$items wa_sessions"
[ -d "$DATA_DIR/uploads" ] && items="$items uploads"
# Ejecutar con MAINTENANCE_MODE=true para que SQLite no esté recibiendo escrituras.
(
  cd "$DATA_DIR"
  tar -czf "$ARCHIVE" $items
)
sha256sum "$ARCHIVE" > "$ARCHIVE.sha256"
echo "Backup creado: $ARCHIVE"
cat "$ARCHIVE.sha256"
