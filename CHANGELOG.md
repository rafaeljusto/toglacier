# Change log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]
### Added
- Subcommands to manage the backups (sync, get, list, remove, start, encrypt)
- Sensitive parameters can now be encrypted

### Fixed
- Use multipart upload strategy when the archive is bigger than 100MB (was 1GB)
- Remove temporary tarball after synchronization

### Changed
- Major refactory on the project structure, with unit tests and documentation

## [1.0.0] - 2016-12-08
### Added
- Allow multiple backup paths
- Uploaded archive checksum validation
- Automatically remove old backups
- Keep track of the backups in an audit file
- Run backup task periodically
- Build and send tarball to AWS Glacier