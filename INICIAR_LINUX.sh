#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"
[ -f .env ] || cp .env.example .env
go run .
