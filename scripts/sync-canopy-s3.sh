#!/usr/bin/env bash
# Builds .chm canopy tiles (ETH Global Canopy Height 2020 masked by ESA WorldCover 2021) into the S3 bucket from .env. Needs curl, python3 and gdalwarp, or a podman machine.
# Usage:
#   scripts/sync-canopy-s3.sh [-j N] [-e ENVFILE] [--] LAT_MIN LAT_MAX LON_MIN LON_MAX   # 1 degree tiles, e.g. 45 47 6 10 (-- before a negative LAT_MIN)
#   scripts/sync-canopy-s3.sh [-j N] [-e ENVFILE] SOURCE...                        # whole 3 degree source tiles, e.g. N45E009
set -uo pipefail

ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env"
LOG_FILE="${CANOPY_SYNC_LOG:-$PWD/canopy-sync.log}"
EXPECTED_SIZE=12967201 # 3601*3601
SOURCE="https://libdrive.ethz.ch/public.php/webdav/3deg_cogs"
SOURCE_AUTH="cO8or7iOe5dT2Rt:" # public share token, no password
MASK="https://esa-worldcover.s3.eu-central-1.amazonaws.com/v200/2021/map"
GDAL_IMAGE="ghcr.io/osgeo/gdal:alpine-small-3.11.3"

JOBS=1
while getopts "j:e:" opt; do
	case $opt in
	j) JOBS=$OPTARG ;;
	e) ENV_FILE=$OPTARG ;;
	*) exit 2 ;;
	esac
done
shift $((OPTIND - 1))

[ -r "$ENV_FILE" ] || { echo "env file not readable: $ENV_FILE" >&2; exit 2; }
envval() { sed -n "s/^$1=//p" "$ENV_FILE" | tail -1; }
ENDPOINT=$(envval RADIOPATH_DEM_S3_ENDPOINT)
BUCKET=$(envval RADIOPATH_DEM_S3_BUCKET)
PREFIX=$(envval RADIOPATH_CANOPY_S3_PREFIX)
REGION=$(envval RADIOPATH_DEM_S3_REGION); REGION=${REGION:-us-east-1}
ACCESS_KEY=$(envval RADIOPATH_DEM_S3_ACCESS_KEY)
SECRET_KEY=$(envval RADIOPATH_DEM_S3_SECRET_KEY)
[ -n "$ENDPOINT" ] && [ -n "$BUCKET" ] && [ -n "$ACCESS_KEY" ] && [ -n "$PREFIX" ] || { echo "RADIOPATH_DEM_S3_ENDPOINT, _BUCKET, _ACCESS_KEY, _SECRET_KEY and RADIOPATH_CANOPY_S3_PREFIX must be set in $ENV_FILE" >&2; exit 2; }

s3curl() { curl -sS --aws-sigv4 "aws:amz:${REGION}:s3" --user "${ACCESS_KEY}:${SECRET_KEY}" "$@"; }

log() { printf '%s %s %s %s\n' "$(date -u +%FT%TZ)" "$1" "$2" "$3" >> "$LOG_FILE"; }

tile_name() {
	local lat=$1 lon=$2 ns=N ew=E
	[ "$lat" -lt 0 ] && { ns=S; lat=$((-lat)); }
	[ "$lon" -lt 0 ] && { ew=W; lon=$((-lon)); }
	printf '%s%02d%s%03d' "$ns" "$lat" "$ew" "$lon"
}

floor3() { echo $(( $1 - (($1 % 3 + 3) % 3) )); }

obj_url() { echo "${ENDPOINT%/}/${BUCKET}/${PREFIX}${1:0:3}/${1}.chm"; }

convert_tile() {
	local lat=$1 lon=$2 tif=$3 cls=$4 tile chm te size code
	tile=$(tile_name "$lat" "$lon")
	chm="$WORKDIR/$tile.chm"
	te=$(awk -v lat="$lat" -v lon="$lon" 'BEGIN { h = 1/7200; printf "%.9f %.9f %.9f %.9f", lon - h, lat - h, lon + 1 + h, lat + 1 + h }')
	# shellcheck disable=SC2086
	if ! $GDAL gdalwarp -q -overwrite -te $te -ts 3601 3601 -r max -srcnodata 255 -dstnodata None -wo INIT_DEST=0 -ot Byte -of ENVI "$tif" "$WORKDIR/$tile.h" ||
		! $GDAL gdalwarp -q -overwrite -te $te -ts 3601 3601 -r mode -srcnodata 0 -dstnodata None -wo INIT_DEST=0 -ot Byte -of ENVI "$cls" "$WORKDIR/$tile.c" ||
		! python3 -c 'import sys
h, c = open(sys.argv[1], "rb").read(), open(sys.argv[2], "rb").read()
open(sys.argv[3], "wb").write(bytes(a if b == 10 else 0 for a, b in zip(h, c)))' "$WORKDIR/$tile.h" "$WORKDIR/$tile.c" "$chm"; then
		rm -f "$WORKDIR/$tile".*
		log "$tile" failed convert
		return 1
	fi
	rm -f "$WORKDIR/$tile".h "$WORKDIR/$tile".c "$WORKDIR/$tile".hdr "$WORKDIR/$tile".h.aux.xml "$WORKDIR/$tile".c.aux.xml
	size=$(wc -c < "$chm" | tr -d ' ')
	if [ "$size" != "$EXPECTED_SIZE" ]; then
		rm -f "$chm"
		log "$tile" failed "bad-size-$size"
		return 1
	fi
	if [ -z "$(tr -d '\000' < "$chm" | head -c 1)" ]; then
		rm -f "$chm"
		log "$tile" no-trees 0
		return 0
	fi
	code=$(s3curl -o /dev/null -w '%{http_code}' -T "$chm" -H "Content-Type: application/octet-stream" "$(obj_url "$tile")")
	rm -f "$chm"
	if [ "$code" != "200" ] && [ "$code" != "201" ]; then
		log "$tile" failed "upload-http-$code"
		return 1
	fi
	log "$tile" uploaded "$size"
}

fetch() {
	local code
	code=$(curl -sS ${3:+-u "$3"} -o "$2" -w '%{http_code}' "$1")
	if [ "$code" != "200" ] && [ "$code" != "404" ]; then
		sleep 10
		code=$(curl -sS ${3:+-u "$3"} -o "$2" -w '%{http_code}' "$1")
	fi
	echo "$code"
}

sync_source() {
	local lat0=$1 lon0=$2 name tif cls lat lon tile size code rc=0 pair
	local todo=()
	name=$(tile_name "$lat0" "$lon0")
	tif="$WORKDIR/$name.tif"
	cls="$WORKDIR/$name.cls.tif"
	for ((lat = lat0 > $3 ? lat0 : $3; lat <= (lat0 + 2 < $4 ? lat0 + 2 : $4); lat++)); do
		for ((lon = lon0 > $5 ? lon0 : $5; lon <= (lon0 + 2 < $6 ? lon0 + 2 : $6); lon++)); do
			tile=$(tile_name "$lat" "$lon")
			grep -q " $tile no-trees " "$LOG_FILE" 2>/dev/null && continue
			size=$(s3curl -I "$(obj_url "$tile")" 2>/dev/null | tr -d '\r' | awk 'tolower($1)=="content-length:" {print $2}')
			if [ "$size" = "$EXPECTED_SIZE" ]; then
				log "$tile" skipped-exists "$size"
				continue
			fi
			todo+=("$lat $lon")
		done
	done
	[ "${#todo[@]}" -eq 0 ] && return 0

	code=$(fetch "$SOURCE/ETH_GlobalCanopyHeight_10m_2020_${name}_Map.tif" "$tif" "$SOURCE_AUTH")
	if [ "$code" = "404" ]; then
		rm -f "$tif"
		log "$name" no-source 0
		return 0
	fi
	if [ "$code" != "200" ]; then
		rm -f "$tif"
		log "$name" failed "download-http-$code"
		return 1
	fi
	code=$(fetch "$MASK/ESA_WorldCover_10m_2021_v200_${name}_Map.tif" "$cls")
	if [ "$code" != "200" ]; then
		rm -f "$tif" "$cls"
		log "$name" failed "mask-http-$code"
		return 1
	fi
	for pair in "${todo[@]}"; do
		# shellcheck disable=SC2086
		convert_tile $pair "$tif" "$cls" || rc=1
	done
	rm -f "$tif" "$cls"
	return $rc
}

if [ "${CANOPY_SYNC_WORKER:-}" = 1 ]; then
	WORKDIR=$CANOPY_SYNC_WORKDIR
	GDAL=$CANOPY_SYNC_GDAL
	fails=$(cat "$WORKDIR/.fails" 2>/dev/null); [[ $fails =~ ^[0-9]+$ ]] || fails=0
	[ "$fails" -ge 3 ] && { echo "abort: 3 consecutive source tiles failed" >&2; exit 255; }
	# shellcheck disable=SC2086
	if sync_source $1; then fails=0; else fails=$((fails + 1)); fi
	echo "$fails" > "$WORKDIR/.fails.$$" && mv -f "$WORKDIR/.fails.$$" "$WORKDIR/.fails"
	exit 0
fi

sources=()
if [ $# -eq 4 ] && [[ $1 =~ ^-?[0-9]+$ ]]; then
	for ((lat0 = $(floor3 "$1"); lat0 <= $2; lat0 += 3)); do
		for ((lon0 = $(floor3 "$3"); lon0 <= $4; lon0 += 3)); do
			sources+=("$lat0 $lon0 $1 $2 $3 $4")
		done
	done
elif [ $# -ge 1 ]; then
	for name in "$@"; do
		lat0=$((10#${name:1:2})); [ "${name:0:1}" = S ] && lat0=$((-lat0))
		lon0=$((10#${name:4:3})); [ "${name:3:1}" = W ] && lon0=$((-lon0))
		sources+=("$lat0 $lon0 $lat0 $((lat0 + 2)) $lon0 $((lon0 + 2))")
	done
else
	sed -n '2,5p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
fi

code=$(s3curl -o /dev/null -w '%{http_code}' -I "${ENDPOINT%/}/${BUCKET}/")
[ "$code" = "200" ] || { echo "bucket check failed: HTTP $code for ${ENDPOINT%/}/${BUCKET}/" >&2; exit 1; }

command -v python3 > /dev/null || { echo "need python3" >&2; exit 2; }
mkdir -p "$HOME/.cache"
WORKDIR=$(mktemp -d "$HOME/.cache/canopy-sync.XXXXXX")
trap 'rm -rf "$WORKDIR"' EXIT
if command -v gdalwarp > /dev/null; then
	GDAL=""
else
	podman info > /dev/null 2>&1 || { echo "need gdalwarp in PATH or a running podman machine" >&2; exit 2; }
	GDAL="podman run --rm -v $WORKDIR:$WORKDIR $GDAL_IMAGE"
fi

echo "syncing ${#sources[@]} source tiles with $JOBS workers, log: $LOG_FILE"
start=$(date +%s)
printf '%s\n' "${sources[@]}" | CANOPY_SYNC_WORKER=1 CANOPY_SYNC_WORKDIR=$WORKDIR CANOPY_SYNC_GDAL="$GDAL" \
	xargs -P "$JOBS" -I{} "$0" -e "$ENV_FILE" {}

echo "done in $(( $(date +%s) - start )) s"
for a in uploaded skipped-exists no-trees no-source failed; do
	printf '%-15s %5d\n' "$a" "$(awk -v a=$a '$3==a' "$LOG_FILE" | grep -c . )"
done
awk '$3=="uploaded" {s+=$4} END {printf "uploaded %.1f GB\n", s/1e9}' "$LOG_FILE"
awk '$3=="failed" {print "FAILED", $2, $4}' "$LOG_FILE"
