#!/usr/bin/env bash
# Runs the unit tests with coverage and prints the coverage of each package as a Markdown
# table, appended to $GITHUB_STEP_SUMMARY when it is set. Fails when the tests fail, or when
# a gated package is below COVERAGE_THRESHOLD percent (80 by default).
#
# Gated packages are those whose import path matches COVERAGE_GATED, an extended regular
# expression: the client packages and the controller packages, without the controller root,
# which only wires the controllers together. A normalization package outside these paths
# must be added to the expression when it lands.
set -euo pipefail

threshold="${COVERAGE_THRESHOLD:-80}"
gated="${COVERAGE_GATED:-/internal/(client(/|$)|controller/)}"

log="$(mktemp)"
trap 'rm -f "${log}"' EXIT

# A failing test still gets its coverage table; the script fails at the end.
go test -count=1 -cover ./... 2>&1 | tee "${log}" && tested=0 || tested=$?

# go test prints one line per package: "coverage: N% of statements" when the package has
# statements, "[no test files]" when it has none to cover.
table="$(awk -v threshold="${threshold}" -v gated="${gated}" '
  BEGIN {
    print "| Package | Coverage | Gate |"
    print "| --- | --- | --- |"
  }
  {
    pkg = ""
    for (i = 1; i <= NF; i++) if ($i ~ /\//) { pkg = $i; break }
    if (pkg == "") next
    if ($0 ~ /\[no test files\]/) cov = "n/a"
    else if (match($0, /coverage: [0-9.]+%/)) cov = substr($0, RSTART + 10, RLENGTH - 11)
    else next
    gate = "-"
    if (pkg ~ gated) {
      if (cov == "n/a") gate = "pass (no statements)"
      else if (cov + 0 >= threshold) gate = "pass (>= " threshold "%)"
      else { gate = "**FAIL** (< " threshold "%)"; failed = 1 }
    }
    printf "| `%s` | %s | %s |\n", pkg, (cov == "n/a" ? cov : cov "%"), gate
  }
  END { exit failed }
' "${log}")" && status=0 || status=$?

{
  echo "### Unit test coverage"
  echo
  echo "${table}"
} | tee -a "${GITHUB_STEP_SUMMARY:-/dev/null}"

if [[ "${status}" -ne 0 ]]; then
  echo "coverage is below ${threshold}% on a gated package" >&2
fi
if [[ "${tested}" -ne 0 ]]; then
  echo "the unit tests failed" >&2
  exit "${tested}"
fi
exit "${status}"
