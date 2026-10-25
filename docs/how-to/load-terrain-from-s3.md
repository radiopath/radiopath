# Load terrain from S3

How to serve the elevation model from an S3-compatible bucket instead of a directory, and how to fill that bucket
with Copernicus GLO-30. The variables are listed in [Configuration](../reference/configuration.md).

With `RADIOPATH_DATASET_S3_ENDPOINT` and `RADIOPATH_DATASET_S3_BUCKET` set, tiles are
fetched on demand with `GET {endpoint}/{bucket}/{dataset}/{band}/{name}.hgt`
(path style, AWS Signature V4 when keys are given) and kept in memory like
directory tiles. Keys are grouped by latitude band, `srtm1/N47/N47E009.hgt`,
so a bucket covering the world stays browsable; directories are flat. `{dataset}` is
`RADIOPATH_DATASET_DEM` (default `srtm1`); the canopy layer and other datasets sit
next to it in the same bucket.
Only GetObject is used, so a read-only policy on the bucket is enough.
`scripts/sync-dem-s3.sh` fills the bucket from Copernicus GLO-30 (public AWS
bucket `copernicus-dem-30m`, one ~40 MB COG per degree) using the S3 settings
in `.env`: each tile is warped onto the 1" grid with gdalwarp together with its
east, south and south-east neighbours, because the source has 3600 pixels per
degree and the 3601st row and column belong to the next tile. Needs curl and
gdalwarp. Resumable, skips sea tiles, `-f` overwrites existing tiles:

```sh
scripts/sync-dem-s3.sh 34 59 -11 44         # Europe: lat 34..59, lon -11..44, about 1200 tiles / 31 GB
scripts/sync-dem-s3.sh -j 4 -- -90 89 -180 179 # the world: about 26 000 tiles / 680 GB, 8 h with 4 workers
scripts/sync-dem-s3.sh -f N47E009 N47E008   # single tiles, replaced
```

The dataset name `srtm1` is historical: the tiles are Copernicus data in the
SRTM file format.

Fetching a 26 MB tile takes a moment on first use; size `RADIOPATH_DEM_CACHE_TILES`
so the working area stays in memory. The readiness probe does not check S3.

## The HF data set

The ITU data of the HF model sits in the same bucket under `RADIOPATH_DATASET_HF` (default `hf1`), flat, as
`hf1/ionos01.bin`, `hf1/COEFF01W.txt`, `hf1/P1239-3 Decile Factors.txt` and so on. Unlike the tiles it is not read
on demand: at startup the server downloads all 25 files (about 131 MB) into a temporary directory, checks each
against its SHA-256 in `internal/hf/data.sha256` and removes the directory on exit; keep that much room in `/tmp`.
To fill the bucket, run `scripts/fetch-hf-data.sh` and copy the folder it creates, for example with
`mc mirror tmp/testfiles/hf1 myminio/radiopath-datasets/hf1`. A bucket without the files, a file that does not match
its checksum or a failed download leave the HF model off; the server logs it and runs without HF.
