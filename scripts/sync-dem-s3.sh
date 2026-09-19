#!/usr/bin/env bash
# Fills the S3 bucket from .env with Copernicus GLO-30 as SRTM-style .hgt tiles. Needs curl >= 7.75 and gdalwarp.
# Usage:
#   scripts/sync-dem-s3.sh [-j N] [-f] [-e ENVFILE] [--] LAT_MIN LAT_MAX LON_MIN LON_MAX   # e.g. 34 59 -11 44 for Europe (-- before a negative LAT_MIN)
#   scripts/sync-dem-s3.sh [-j N] [-f] [-e ENVFILE] TILE...                          # e.g. N47E009 N47E008
set -uo pipefail

JOBS=4
FORCE=0
ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env"
LOG_FILE="${DEM_SYNC_LOG:-$PWD/dem-sync.log}"
EXPECTED_SIZE=25934402 # 3601*3601*2
FAIL_LIMIT=10
SOURCE="https://copernicus-dem-30m.s3.amazonaws.com"

while getopts "j:fe:" opt; do
	case $opt in
	j) JOBS=$OPTARG ;;
	f) FORCE=1 ;;
	e) ENV_FILE=$OPTARG ;;
	*) exit 2 ;;
	esac
done
shift $((OPTIND - 1))

[ -r "$ENV_FILE" ] || { echo "env file not readable: $ENV_FILE" >&2; exit 2; }
envval() { sed -n "s/^$1=//p" "$ENV_FILE" | tail -1; }
ENDPOINT=$(envval RADIOPATH_DEM_S3_ENDPOINT)
BUCKET=$(envval RADIOPATH_DEM_S3_BUCKET)
PREFIX=$(envval RADIOPATH_DEM_S3_PREFIX)
REGION=$(envval RADIOPATH_DEM_S3_REGION); REGION=${REGION:-us-east-1}
ACCESS_KEY=$(envval RADIOPATH_DEM_S3_ACCESS_KEY)
SECRET_KEY=$(envval RADIOPATH_DEM_S3_SECRET_KEY)
[ -n "$ENDPOINT" ] && [ -n "$BUCKET" ] && [ -n "$ACCESS_KEY" ] || { echo "RADIOPATH_DEM_S3_ENDPOINT, _BUCKET, _ACCESS_KEY and _SECRET_KEY must be set in $ENV_FILE" >&2; exit 2; }

s3curl() { curl -sS --aws-sigv4 "aws:amz:${REGION}:s3" --user "${ACCESS_KEY}:${SECRET_KEY}" "$@"; }

log() { printf '%s %s %s %s\n' "$(date -u +%FT%TZ)" "$1" "$2" "$3" >> "$LOG_FILE"; }

tile_name() {
	local lat=$1 lon=$2 ns=N ew=E
	[ "$lat" -lt 0 ] && { ns=S; lat=$((-lat)); }
	[ "$lon" -lt 0 ] && { ew=W; lon=$((-lon)); }
	printf '%s%02d%s%03d' "$ns" "$lat" "$ew" "$lon"
}

source_url() {
	local name="Copernicus_DSM_COG_10_${1:0:3}_00_${1:3:4}_00_DEM"
	echo "$SOURCE/$name/$name.tif"
}

fetch_source() {
	local code
	code=$(curl -sS -o "$2" -w '%{http_code}' "$(source_url "$1")")
	if [ "$code" != "200" ] && [ "$code" != "404" ] && [ "$code" != "403" ]; then
		sleep 10
		code=$(curl -sS -o "$2" -w '%{http_code}' "$(source_url "$1")")
	fi
	[ "$code" = "403" ] && code=404
	[ "$code" = "200" ] || rm -f "$2"
	echo "$code"
}

sync_tile() {
	local tile=$1 work=$2
	local obj="${ENDPOINT%/}/${BUCKET}/${PREFIX}${tile:0:3}/${tile}.hgt"
	local tif="$work/$tile.tif" hgt="$work/$tile.hgt"
	local size code lat lon te n
	local inputs=("$tif")

	if [ "$FORCE" = 0 ]; then
		size=$(s3curl -I "$obj" 2>/dev/null | tr -d '\r' | awk 'tolower($1)=="content-length:" {print $2}')
		if [ "$size" = "$EXPECTED_SIZE" ]; then
			log "$tile" skipped-exists "$size"
			return 0
		fi
	fi

	code=$(fetch_source "$tile" "$tif")
	if [ "$code" = "404" ]; then
		log "$tile" no-tile 0
		return 0
	fi
	if [ "$code" != "200" ]; then
		log "$tile" failed "download-http-$code"
		return 1
	fi

	lat=$((10#${tile:1:2})); [ "${tile:0:1}" = S ] && lat=$((-lat))
	lon=$((10#${tile:4:3})); [ "${tile:3:1}" = W ] && lon=$((-lon))
	for n in "$lat $((lon == 179 ? -180 : lon + 1))" "$((lat - 1)) $lon" "$((lat - 1)) $((lon == 179 ? -180 : lon + 1))"; do
		# shellcheck disable=SC2086
		n=$(tile_name $n)
		code=$(fetch_source "$n" "$work/$tile.$n.tif")
		case $code in
		200) inputs+=("$work/$tile.$n.tif") ;;
		404) ;;
		*) rm -f "$work/$tile".*; log "$tile" failed "neighbour-$n-http-$code"; return 1 ;;
		esac
	done

	te=$(awk -v lat="$lat" -v lon="$lon" 'BEGIN { h = 1/7200; printf "%.9f %.9f %.9f %.9f", lon - h, lat - h, lon + 1 + h, lat + 1 + h }')
	# shellcheck disable=SC2086
	if ! gdalwarp -q -overwrite -te $te -ts 3601 3601 -r bilinear -ot Int16 -of SRTMHGT "${inputs[@]}" "$hgt" 2>/dev/null; then
		rm -f "$work/$tile".*
		log "$tile" failed convert
		return 1
	fi
	rm -f "${inputs[@]}" "$hgt.aux.xml"
	size=$(wc -c < "$hgt" | tr -d ' ')
	if [ "$size" != "$EXPECTED_SIZE" ]; then
		rm -f "$hgt"
		log "$tile" failed "bad-size-$size"
		return 1
	fi

	code=$(s3curl -o /dev/null -w '%{http_code}' -T "$hgt" -H "Content-Type: application/octet-stream" "$obj")
	rm -f "$hgt"
	if [ "$code" != "200" ] && [ "$code" != "201" ]; then
		log "$tile" failed "upload-http-$code"
		return 1
	fi
	log "$tile" uploaded "$size"
}

if [ "${DEM_SYNC_WORKER:-}" = 1 ]; then
	work=$DEM_SYNC_WORKDIR
	fails=$(cat "$work/.fails" 2>/dev/null); [[ $fails =~ ^[0-9]+$ ]] || fails=0
	[ "$fails" -ge "$FAIL_LIMIT" ] && { echo "abort: $FAIL_LIMIT consecutive failures" >&2; exit 99; }
	if sync_tile "$1" "$work"; then fails=0; else fails=$((fails + 1)); fi
	echo "$fails" > "$work/.fails.$$" && mv -f "$work/.fails.$$" "$work/.fails"
	exit 0
fi

tiles=()
if [ $# -eq 4 ] && [[ $1 =~ ^-?[0-9]+$ ]]; then
	for ((lat = $1; lat <= $2; lat++)); do
		for ((lon = $3; lon <= $4; lon++)); do
			tiles+=("$(tile_name "$lat" "$lon")")
		done
	done
elif [ $# -ge 1 ]; then
	tiles=("$@")
else
	sed -n '2,5p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
fi

command -v gdalwarp > /dev/null || { echo "need gdalwarp in PATH" >&2; exit 2; }
code=$(s3curl -o /dev/null -w '%{http_code}' -I "${ENDPOINT%/}/${BUCKET}/")
[ "$code" = "200" ] || { echo "bucket check failed: HTTP $code for ${ENDPOINT%/}/${BUCKET}/" >&2; exit 1; }

WORKDIR=$(mktemp -d "${TMPDIR:-/tmp}/dem-sync.XXXXXX")
trap 'rm -rf "$WORKDIR"' EXIT
echo "syncing ${#tiles[@]} tiles with $JOBS workers, log: $LOG_FILE"
start=$(date +%s)
force_flag=(); [ "$FORCE" = 1 ] && force_flag=(-f)
printf '%s\n' "${tiles[@]}" | DEM_SYNC_WORKER=1 DEM_SYNC_WORKDIR=$WORKDIR xargs -P "$JOBS" -n 1 "$0" -e "$ENV_FILE" "${force_flag[@]}"

echo "done in $(( $(date +%s) - start )) s"
for a in uploaded skipped-exists no-tile failed; do
	printf '%-15s %5d\n' "$a" "$(awk -v a=$a '$3==a' "$LOG_FILE" | grep -c . )"
done
awk '$3=="uploaded" {s+=$4} END {printf "uploaded %.1f GB\n", s/1e9}' "$LOG_FILE"
awk '$3=="failed" {print "FAILED", $2, $4}' "$LOG_FILE"
