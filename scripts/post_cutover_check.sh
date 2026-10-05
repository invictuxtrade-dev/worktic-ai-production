#!/bin/sh
set -eu
BASE_URL="${BASE_URL:?BASE_URL requerido}"
echo "[1/3] healthz"
curl -fsS "$BASE_URL/healthz"; echo
echo "[2/3] readyz"
curl -fsS "$BASE_URL/readyz"; echo
echo "[3/3] login page"
curl -fsSI "$BASE_URL/login.html" | head -n 1
echo "Checks básicos OK"
