#!/usr/bin/env bash

set -euo pipefail

threshold="${COVERAGE_THRESHOLD:-95}"
profile="${1:-coverage.out}"

go test -coverprofile="${profile}" ./...

total="$(go tool cover -func="${profile}" | awk '/^total:/ {gsub("%", "", $3); print $3}')"

awk -v actual="${total}" -v threshold="${threshold}" '
BEGIN {
  if (actual + 0 < threshold + 0) {
    printf("coverage %.1f%% is below threshold %.1f%%\n", actual, threshold)
    exit 1
  }

  printf("coverage %.1f%% meets threshold %.1f%%\n", actual, threshold)
}'
