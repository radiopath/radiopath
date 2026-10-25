#!/usr/bin/env bash
# Downloads the ITU-R data set of the HF model (P.533 / P.372) from the ITU-R Study Group 3 repository, pinned to one
# commit and verified by SHA-256, so an upstream change can never alter the model silently. About 131 MB.
# Usage:
#   scripts/fetch-hf-data.sh [-f] [DATASET_DIR]    # into DATASET_DIR/hf1; DATASET_DIR defaults to $RADIOPATH_DATASET_DIR, else the one in .env, else tmp/testfiles (dev)
# Files that already match their SHA-256 are skipped, -f downloads everything again.
# To move to a newer upstream: change COMMIT, run with -f after replacing the hashes in internal/hf/data.sha256
# (sha256sum of the new files), and run `go test ./internal/hf/...` against the result.
set -euo pipefail

REPO="ITU-R-Study-Group-3/ITU-R-HF"
COMMIT="1842db683f3f19146e60c77302f79172dfab0b7a" # last commit that touched P533/Data (2019-02-27)
SOURCE="https://raw.githubusercontent.com/$REPO/$COMMIT/P533/Data"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# SHA-256 of every file, "hash  name", the list the server verifies its S3 download against
FILES=$(cat "$ROOT/internal/hf/data.sha256")

FORCE=0
while getopts "f" opt; do
	case $opt in
	f) FORCE=1 ;;
	*) exit 2 ;;
	esac
done
shift $((OPTIND - 1))

DATASET=${1:-${RADIOPATH_DATASET_DIR:-}}
if [ -z "$DATASET" ] && [ -r "$ROOT/.env" ]; then
	DATASET=$(sed -n 's/^RADIOPATH_DATASET_DIR=//p' "$ROOT/.env" | tail -1)
fi
DATASET=${DATASET:-tmp/testfiles}
# a relative path, as in .env, is relative to the repository
case "$DATASET" in /*) ;; *) DATASET=$ROOT/$DATASET ;; esac
DIR=$DATASET/hf1
mkdir -p "$DIR"

sha256() {
	if command -v sha256sum > /dev/null; then sha256sum < "$1" | cut -d' ' -f1; else shasum -a 256 < "$1" | cut -d' ' -f1; fi
}

while IFS= read -r line; do
	want=${line%%  *}
	name=${line#*  }
	file="$DIR/$name"
	if [ "$FORCE" = 0 ] && [ -f "$file" ] && [ "$(sha256 "$file")" = "$want" ]; then
		echo "ok     $name"
		continue
	fi
	echo "fetch  $name"
	curl -fsS --retry 3 -o "$file.part" "$SOURCE/${name// /%20}"
	got=$(sha256 "$file.part")
	if [ "$got" != "$want" ]; then
		rm -f "$file.part"
		echo "checksum mismatch for $name: got $got, want $want" >&2
		exit 1
	fi
	mv -f "$file.part" "$file"
done <<< "$FILES"
echo "done: $DIR (set RADIOPATH_DATASET_DIR to $DATASET)"
