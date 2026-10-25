#!/usr/bin/env bash
# Builds the unmodified ITU-R C reference (P.533, P.372, ITURHFProp) with gcc and checks that it still produces the
# committed reference reports; run by the CI job hf-reference, see README.txt.
# Usage: check-reference.sh ITU_SOURCE_DIR DATA_DIR
#   ITU_SOURCE_DIR  checkout of https://github.com/ITU-R-Study-Group-3/ITU-R-HF (P533/Src, P372/Src, ITURHFProp/Src)
#   DATA_DIR        data set of scripts/fetch-hf-data.sh
# Build flags are those of the upstream Linux Makefiles except -fcommon for -z muldefs, and MakeNoise.c in the noise
# library (the Linux Makefile leaves it out, the Visual Studio project has it). errno.h is force-included because
# MakeNoise.c uses errno without including its declaration. The sources are not patched.
set -euo pipefail

abs() { (cd "$1" && { pwd -W 2> /dev/null || pwd; }); }
SRC=$(abs "$1")
HERE=$(abs "$(dirname "${BASH_SOURCE[0]}")")
BUILD=$(mktemp -d)
OUT=$(mktemp -d)
trap 'rm -rf "$BUILD" "$OUT"' EXIT

case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN*) LIB533=P533.dll LIB372=P372.dll EXE=ITURHFProp.exe ;; # the program loads these names
*) LIB533=libp533.so LIB372=libp372.so EXE=ITURHFProp ;;
esac
CFLAGS=(-O2 -fPIC -fcommon -w)

(
	cd "$SRC"
	gcc "${CFLAGS[@]}" -include errno.h -shared -o "$BUILD/$LIB372" P372/Src/P372/{InitializeNoise,Noise,NoiseMemory,MakeNoise}.c -IP372/Src/P372 -lm
	gcc "${CFLAGS[@]}" -std=c99 -shared -o "$BUILD/$LIB533" P533/Src/P533/*.c -IP533/Src/P533 -lm -ldl
	gcc "${CFLAGS[@]}" -o "$BUILD/$EXE" ITURHFProp/Src/ITURHFProp/*.c -IITURHFProp/Src/ITURHFProp -lm -ldl
)

LD_LIBRARY_PATH="$BUILD${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" "$HERE/regen.sh" "$BUILD/$EXE" "$2" "$OUT" > /dev/null

# The reports carry the build and run time; everything else must be identical.
strip() { tr -d '\r' < "$1" | grep -vE 'ITURHFProp +Ver |Analysis Prepared'; }
bad=0
for ref in "$HERE"/*.csv; do
	name=$(basename "$ref")
	if ! diff <(strip "$ref") <(strip "$OUT/$name") > "$OUT/$name.diff"; then
		echo "differs: $name"
		head -n 10 "$OUT/$name.diff"
		bad=1
	fi
done
[ "$bad" = 0 ] && echo "all $(ls "$HERE"/*.csv | wc -l) reference reports match the C program"
exit "$bad"
