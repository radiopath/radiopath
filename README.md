# Radiopath

> [!TIP]
> **Just want to plan a link? Use the hosted instance: [app.radiopath.org](https://app.radiopath.org)**
> — no installation, free to sign up. The rest of this README is about running and developing Radiopath.

Radio coverage and link planning for amateur radio and beyond. Web application
written in Go, inspired by [Radio Mobile Online by VE2DBE](https://www.ve2dbe.com/rmonline_s.asp).
Propagation is computed with the NTIA
Irregular Terrain Model (Longley-Rice), ported to Go from the public domain
reference implementation (see `internal/itm/NOTICE`).

Current scope: point-to-point link analysis (terrain profile, ITM path loss,
link budget, first Fresnel zone clearance) and area coverage (link margin raster
around a transmitter, computed along radials and stored as PNG).

## Running Radiopath

You need PostgreSQL with PostGIS, terrain tiles in the SRTM `.hgt` format (in a directory or an S3 bucket) and,
strongly recommended, Valkey or Redis for the map tile cache. Radiopath is one binary, also published as the image
`ghcr.io/radiopath/radiopath`, configured entirely through environment variables. For Kubernetes there is a Helm
chart in [radiopath/helmchart](https://github.com/radiopath/helmchart).

- [Configuration](docs/reference/configuration.md): every environment variable
- [Operations](docs/reference/operations.md): requirements, probes, memory, map tiles, managing users
- [Load terrain from S3](docs/how-to/load-terrain-from-s3.md) and [build the tree canopy layer](docs/how-to/build-the-canopy-layer.md)
- [Metrics and logging](docs/reference/metrics-and-logging.md)

## Documentation

The full documentation is in [docs/](docs/README.md), organised by the [Diátaxis](https://diataxis.fr) framework:
tutorials to learn, how-to guides to get things done, reference to look things up, and explanation to understand
the design, from [the propagation model](docs/explanation/propagation-model.md) to the
[architecture](docs/explanation/architecture.md).

## Contributing

Start with the [development tutorial](docs/tutorials/getting-started.md); the host needs only Go and Docker:

```sh
make dev    # http://localhost:5173, login dev / devpassword; Ctrl-C stops everything
```

[CONTRIBUTING.md](CONTRIBUTING.md) describes the conventions, the checks a change needs and how the changelog is
kept.

## License

AGPL-3.0-or-later, see `LICENSE`: use, modify, self-host and redistribute
freely, as long as the source of your version is published under the same
license, also when it is only offered over a network. If you want to run a
modified Radiopath as a hosted service without publishing your changes, a
commercial license is available, see `COMMERCIAL-LICENSE.md`.

The ITM port in `internal/itm` derives from NTIA public domain software, see
`internal/itm/NOTICE`. Bundled into the app:
Leaflet 1.9.4 (BSD-2), Lucide icons (ISC), JetBrains Mono 2.304 and IBM Plex
Sans (both SIL OFL 1.1, `web/static/fonts/OFL-*.txt`), the Natural Earth 110m
land outline (public domain, `web/src/lib/land.json`).

Data: terrain from the Copernicus DEM GLO-30 (© DLR e.V. 2010-2014 and
© Airbus Defence and Space GmbH 2014-2018, provided under COPERNICUS by the
European Union and ESA, all rights reserved; free use with this attribution),
canopy from ETH Global Canopy Height 2020 and ESA WorldCover 2021 (both
CC BY 4.0, see "Tree canopy"), map tiles © OpenStreetMap contributors (ODbL).
