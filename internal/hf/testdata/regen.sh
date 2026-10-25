#!/usr/bin/env bash
# Regenerates the reference reports (*.csv) with the original ITU-R C program; see README.txt.
# Usage: regen.sh ITURHFPROP_BINARY DATA_DIR [OUT_DIR]    # OUT_DIR defaults to this directory
# The tokens @DATA@ (data directory of scripts/fetch-hf-data.sh) and @TESTDATA@ (this directory) in the .in files are
# filled in on a temporary copy, so the committed inputs stay free of machine paths.
set -euo pipefail

abs() { (cd "$1" && { pwd -W 2> /dev/null || pwd; }); } # pwd -W: native path under Git bash on Windows
BIN=$(abs "$(dirname "$1")")/$(basename "$1")
DATA=$(abs "$2")
HERE=$(abs "$(dirname "${BASH_SOURCE[0]}")")
OUT=$HERE
if [ -n "${3:-}" ]; then
	mkdir -p "$3"
	OUT=$(abs "$3")
fi
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

for in in "$HERE"/*.in; do
	name=$(basename "$in" .in)
	sed -e "s|@DATA@|$DATA|g" -e "s|@TESTDATA@|$HERE|g" "$in" > "$WORK/$name.in"
	(cd "$(dirname "$BIN")" && "$BIN" -s "$WORK/$name.in" "$OUT/$name.csv")
	echo "$name"
done
