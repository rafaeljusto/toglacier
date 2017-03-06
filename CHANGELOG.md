# Change log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [2.0.2] - 2017-03-06
### Fixed
- Fix content range format in multipart strategy
- Fix hash calculation (tree hash) of the uploaded archive
- Check if the audit file exists when listing it
- Remove backup when checksum does not match
- Allow to backup only one file

### Added
- Verifies the hash of each uploaded part in multipart strategy

## [2.0.1] - 2017-03-02
### Fixed
- Default multipart part size in bytes

## [2.0.0] - 2017-02-16
### Added
- Subcommands to manage the backups (sync, get, list, remove, start, encrypt)
- Sensitive parameters can now be encrypted
- Periodically request remote backups information
- Support to YAML configuration file

### Fixed
- Use multipart upload strategy when the archive is bigger than 100MB (was 1GB)
- Remove temporary tarball after synchronization

### Changed
- Major refactory on the project structure, with unit tests and documentation
- Local storage is synchronized when the remote backups information is requested
- New environment variable names for AWS parameters (added TOGLACIER prefix)

## [1.0.0] - 2016-12-08
### Added
- Allow multiple backup paths
- Uploaded archive checksum validation
- Automatically remove old backups
- Keep track of the backups in an audit file
- Run backup task periodically
- Build and send tarball to AWS Glacier