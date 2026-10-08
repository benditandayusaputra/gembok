#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
REV="${1:-gembok}"
quartus_sh --flow compile gembok -c "$REV"
echo
echo "== Ringkasan fitter =="
cat "output_files/$REV.fit.summary"
echo
echo "== Ringkasan timing =="
cat "output_files/$REV.sta.summary"
