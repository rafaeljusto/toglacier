# Change log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]
### Added
- Report e-mail in HTML format
- Option to ignore backup parts from files that didn't change in disk
- Download many archives in parallel
- Using context from AWS library for user cancellation
- Detect ransomware infection (maximum number of modified files)
- Ignore files or directories in the backup paths
- New program to convert local storage format
- Documentation to the YAML configuration file

### Fixed
- Ignore unmodified files when choosing the backup parts to download
- Handle user cancel signal when downloading an archive
- Remove tarball after extraction to save disk space
- Synchronize local storage with archive information after the download
- Detect when the backup ID does not exist
- Search backups algorithm was being used wrongly
- Windows does not support syscall.SIGSTOP
- Don't read environment variables without prefix
- Keep consistency of local storage when removing a backup
- Tool version automatically defined in build scripts
- TAR header path was including volume name in Windows environments

### Changed
- Tool moved to cmd subdirectory
- Optionally list backups containing specific pattern
- Commands now have verbose flag to show/hide internal messages
- File checksum method added to archive.Archive (old archive.Builder) interface
- Backups from internal library are now ordered by ID
- When username and/or password aren't provided no e-mail auth is performed

## [3.0.0] - 2017-05-15
### Added
- Encrypt/decrypt backup with a shared secret
- Encryption data authentication (HMAC-SHA256)
- Send report with the scheduler actions periodically
- Output to a log file using logrus library
- Log verbosity defined in configuration
- BoltDB storage for tracking backups locally
- Local backup now has archive size information
- Graceful shutdown when an OS signal is detected
- Incremental backups (unmodified files are ignored)

### Fixed
- Add sample configuration file to deb and txz packages
- Debian script for cross compilation environments
- Don't remove local backup with less than 24 hours

### Changed
- Archive algorithm refactory to simplify the tar file
- Internal API now has well defined errors
- Variable `TOGLACIER_AUDIT` renamed to `TOGLACIER_DB_FILE`
- Command get now extracts automatically to the current directory

## [2.0.4] - 2017-04-19
### Fixed
- Use multipart upload when the archive is bigger than 100MB (was 100KB)

## [2.0.3] - 2017-03-24
### Fixed
- Fix backup removal on checksum mismatch

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