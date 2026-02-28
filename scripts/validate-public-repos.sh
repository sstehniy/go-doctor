#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_LIST="${ROOT_DIR}/docs/validation/public-repos.txt"
VALIDATION_ROOT="${ROOT_DIR}/artifacts/validation/real-world"
CLONE_ROOT="$(mktemp -d)"

mkdir -p "${VALIDATION_ROOT}"
trap 'rm -rf "${CLONE_ROOT}"' EXIT

cd "${ROOT_DIR}"
go build -o "${ROOT_DIR}/go-doctor" ./cmd/go-doctor

SUMMARY_CSV="${VALIDATION_ROOT}/summary.csv"
echo "repo,runtime_seconds,diagnostic_count,error_count,warning_count,noisy_rules,demote_or_disable_candidates,threshold_tuning_notes,baseline_adoption_friction" > "${SUMMARY_CSV}"

while IFS= read -r repo; do
  [[ -z "${repo}" ]] && continue

  name="${repo##*/}"
  out_dir="${VALIDATION_ROOT}/${name}"
  mkdir -p "${out_dir}"

  clone_url="https://${repo}.git"
  git clone --depth 1 "${clone_url}" "${CLONE_ROOT}/${name}"

  start_epoch="$(date +%s)"
  "${ROOT_DIR}/go-doctor" --format json --output "${out_dir}/result.json" --fail-on none "${CLONE_ROOT}/${name}" || true
  end_epoch="$(date +%s)"

  runtime="$((end_epoch - start_epoch))"
  diagnostics="$(jq '.diagnostics | length' "${out_dir}/result.json")"
  errors="$(jq '[.diagnostics[]? | select(.severity == "error" and (.suppressed // false | not))] | length' "${out_dir}/result.json")"
  warnings="$(jq '[.diagnostics[]? | select(.severity == "warning" and (.suppressed // false | not))] | length' "${out_dir}/result.json")"

  echo "${repo},${runtime},${diagnostics},${errors},${warnings},,,," >> "${SUMMARY_CSV}"
done < "${REPO_LIST}"
