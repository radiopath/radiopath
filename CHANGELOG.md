# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Every commit to main branch now builds a :staging image for the staging environment (by @HB9HIL)

### Changed
- Sessions now expire seven days after they were last used instead of thirty days after login, with thirty days from login as a hard cap. Existing sessions are pulled into the shorter window the next time they are used. (by @HB9HIL)

### Fixed
- The ci pipline was still on push to master, fixed to main (by @HB9HIL)

### Chore
- Moved stuff in the repo around to make it a bit more organized (by @HB9HIL)

## [1.0.0] - 2026-09-19

### Added
- Initial public release: point-to-point link analysis and area coverage with the NTIA Irregular Terrain Model, antenna patterns, tree canopy loss, share links, PDF reports, admin panel, Prometheus metrics and a Helm chart. (by @HB9HIL)
