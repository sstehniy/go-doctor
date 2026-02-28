#!/usr/bin/env bash

set -euo pipefail

mode="${1:-}"

install() {
  local repository version os_name arch_name api_url release_json asset_url archive_path extract_dir bin_dir bin_name binary_path

  repository="${INPUT_REPOSITORY:-stanislavstehniy/go-doctor}"
  version="${INPUT_VERSION:-latest}"

  for required in curl jq find tar; do
    if ! command -v "$required" >/dev/null 2>&1; then
      echo "Missing required command: $required" >&2
      exit 1
    fi
  done

  case "${RUNNER_OS:-}" in
    Linux) os_name="linux" ;;
    macOS) os_name="darwin" ;;
    Windows) os_name="windows" ;;
    *)
      echo "Unsupported runner OS: ${RUNNER_OS:-unknown}" >&2
      exit 1
      ;;
  esac

  case "${RUNNER_ARCH:-}" in
    X64) arch_name="amd64" ;;
    ARM64) arch_name="arm64" ;;
    *)
      echo "Unsupported runner arch: ${RUNNER_ARCH:-unknown}" >&2
      exit 1
      ;;
  esac

  if [[ "$version" == "latest" ]]; then
    api_url="https://api.github.com/repos/${repository}/releases/latest"
  else
    api_url="https://api.github.com/repos/${repository}/releases/tags/${version}"
  fi

  if [[ -n "${INPUT_GITHUB_TOKEN:-}" ]]; then
    release_json="$(curl -fsSL \
      -H "Authorization: Bearer ${INPUT_GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github+json" \
      "${api_url}")"
  else
    release_json="$(curl -fsSL \
      -H "Accept: application/vnd.github+json" \
      "${api_url}")"
  fi

  asset_url="$(echo "${release_json}" | jq -r --arg os "$os_name" --arg arch "$arch_name" '
    .assets
    | map(select(.name | test("go-doctor.*" + $os + ".*" + $arch + ".*(tar\\.gz|zip)$"; "i")))
    | .[0].browser_download_url // empty
  ')"
  if [[ -z "$asset_url" ]]; then
    echo "No matching release asset for ${os_name}/${arch_name} in ${repository}@${version}" >&2
    echo "Available assets:" >&2
    echo "${release_json}" | jq -r '.assets[].name' >&2
    exit 1
  fi

  archive_path="${RUNNER_TEMP}/go-doctor-release-archive"
  extract_dir="${RUNNER_TEMP}/go-doctor-release-extract"
  rm -rf "$extract_dir"
  mkdir -p "$extract_dir"

  if [[ -n "${INPUT_GITHUB_TOKEN:-}" ]]; then
    curl -fsSL \
      -H "Authorization: Bearer ${INPUT_GITHUB_TOKEN}" \
      -H "Accept: application/octet-stream" \
      "$asset_url" \
      -o "$archive_path"
  else
    curl -fsSL "$asset_url" -o "$archive_path"
  fi

  if [[ "$asset_url" == *.zip ]]; then
    if ! command -v unzip >/dev/null 2>&1; then
      echo "Missing required command: unzip" >&2
      exit 1
    fi
    unzip -q "$archive_path" -d "$extract_dir"
  else
    tar -xzf "$archive_path" -C "$extract_dir"
  fi

  bin_name="go-doctor"
  if [[ "$os_name" == "windows" ]]; then
    bin_name="go-doctor.exe"
  fi

  binary_path="$(find "$extract_dir" -type f \( -name "$bin_name" -o -name "go-doctor" -o -name "go-doctor.exe" \) | head -n 1)"
  if [[ -z "$binary_path" ]]; then
    echo "go-doctor binary not found in release archive" >&2
    exit 1
  fi

  bin_dir="${RUNNER_TEMP}/go-doctor-bin"
  mkdir -p "$bin_dir"
  cp "$binary_path" "$bin_dir/$bin_name"
  chmod +x "$bin_dir/$bin_name" || true
  echo "$bin_dir" >> "$GITHUB_PATH"
}

scan() {
  local directory format fail_on diff_base config_path sarif_path json_path
  local findings errors warnings score scan_exit json_exit
  local -a sarif_cmd json_cmd

  directory="${INPUT_DIRECTORY:-.}"
  format="${INPUT_FORMAT:-sarif}"
  fail_on="${INPUT_FAIL_ON:-error}"
  diff_base="${INPUT_DIFF:-}"
  config_path="${INPUT_CONFIG:-}"

  case "$format" in
    sarif|json) ;;
    *)
      echo "Unsupported format input: $format (supported: sarif, json)" >&2
      exit 1
      ;;
  esac

  for required in jq go-doctor; do
    if ! command -v "$required" >/dev/null 2>&1; then
      echo "Missing required command: $required" >&2
      exit 1
    fi
  done

  sarif_path="${RUNNER_TEMP}/go-doctor/results.sarif"
  mkdir -p "$(dirname "$sarif_path")"

  sarif_cmd=(go-doctor --format sarif --output "$sarif_path" --fail-on "$fail_on")
  if [[ -n "$diff_base" ]]; then
    sarif_cmd+=(--diff "$diff_base")
  fi
  if [[ -n "$config_path" ]]; then
    sarif_cmd+=(--config "$config_path")
  fi
  sarif_cmd+=("$directory")

  set +e
  "${sarif_cmd[@]}"
  scan_exit=$?
  set -e

  findings=0
  errors=0
  warnings=0
  score=""

  if [[ -f "$sarif_path" ]]; then
    findings="$(jq '.runs[0].results | length' "$sarif_path")"
    errors="$(jq '[.runs[0].results[]? | select(.level == "error")] | length' "$sarif_path")"
    warnings="$(jq '[.runs[0].results[]? | select(.level == "warning")] | length' "$sarif_path")"
    score="$(jq -r '.runs[0].properties.goDoctorScore // empty' "$sarif_path")"
  fi

  if [[ "$format" == "json" ]]; then
    json_path="${RUNNER_TEMP}/go-doctor/results.json"
    json_cmd=(go-doctor --format json --output "$json_path" --fail-on "$fail_on")
    if [[ -n "$diff_base" ]]; then
      json_cmd+=(--diff "$diff_base")
    fi
    if [[ -n "$config_path" ]]; then
      json_cmd+=(--config "$config_path")
    fi
    json_cmd+=("$directory")

    set +e
    "${json_cmd[@]}"
    json_exit=$?
    set -e

    if [[ -f "$json_path" ]]; then
      findings="$(jq '.diagnostics | length' "$json_path")"
      errors="$(jq '[.diagnostics[]? | select(.severity == "error")] | length' "$json_path")"
      warnings="$(jq '[.diagnostics[]? | select(.severity == "warning")] | length' "$json_path")"
      score="$(jq -r 'if .score and .score.enabled then (.score.value | tostring) else "" end' "$json_path")"
    fi
    if [[ "$scan_exit" -eq 0 && "$json_exit" -ne 0 ]]; then
      scan_exit="$json_exit"
    fi
  fi

  if [[ -f "$sarif_path" ]]; then
    echo "sarif-path=$sarif_path" >> "$GITHUB_OUTPUT"
  else
    echo "sarif-path=" >> "$GITHUB_OUTPUT"
  fi
  echo "score=$score" >> "$GITHUB_OUTPUT"
  echo "findings=$findings" >> "$GITHUB_OUTPUT"
  echo "errors=$errors" >> "$GITHUB_OUTPUT"
  echo "warnings=$warnings" >> "$GITHUB_OUTPUT"
  echo "scan-exit-code=$scan_exit" >> "$GITHUB_OUTPUT"
}

case "$mode" in
  install)
    install
    ;;
  scan)
    scan
    ;;
  *)
    echo "Usage: $0 {install|scan}" >&2
    exit 1
    ;;
esac
