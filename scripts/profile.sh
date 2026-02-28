#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ARTIFACT_DIR="${ROOT_DIR}/artifacts/profiles"
mkdir -p "${ARTIFACT_DIR}"

export GOCACHE="$(mktemp -d)"
trap 'rm -rf "${GOCACHE}"' EXIT

cd "${ROOT_DIR}"

go test ./pkg/godoctor \
  -run '^$' \
  -bench HardeningFixtures \
  -benchmem \
  -cpuprofile "${ARTIFACT_DIR}/hardening.cpu.pprof" \
  -memprofile "${ARTIFACT_DIR}/hardening.mem.pprof" \
  | tee "${ARTIFACT_DIR}/hardening.bench.txt"
