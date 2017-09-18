[![GoDoc](https://godoc.org/github.com/rafaeljusto/toglacier?status.png)](https://godoc.org/github.com/rafaeljusto/toglacier)
[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/rafaeljusto/toglacier/master/LICENSE)
[![Build Status](https://travis-ci.org/rafaeljusto/toglacier.png?branch=master)](https://travis-ci.org/rafaeljusto/toglacier)
[![Coverage Status](https://coveralls.io/repos/github/rafaeljusto/toglacier/badge.svg?branch=master)](https://coveralls.io/github/rafaeljusto/toglacier?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/rafaeljusto/toglacier)](https://goreportcard.com/report/github.com/rafaeljusto/toglacier)
[![codebeat badge](https://codebeat.co/badges/f772925e-c2b3-4631-9f19-83cca1dc3a4b)](https://codebeat.co/projects/github-com-rafaeljusto-toglacier)

![toglacier](https://raw.githubusercontent.com/rafaeljusto/toglacier/master/toglacier.png)

# toglacier
Send data to the cloud periodically.

## What?

Have you ever thought that your server could have some backup in the cloud to
mitigate some crazy [ransomware](https://en.wikipedia.org/wiki/Ransomware)
infection? Great! Here is a peace of software to help you do that, sending your
data periodically to the cloud. For now it could be the [Amazon
Glacier](https://aws.amazon.com/glacier/) or the [Google Cloud
Storage](https://cloud.google.com/storage/archival/) services. It uses the [AWS
SDK](https://aws.amazon.com/sdk-for-go/) and [Google Cloud
SDK](https://github.com/GoogleCloudPlatform/google-cloud-go) behind the scenes,
all honors go to [Amazon developers](https://github.com/orgs/aws/people) and
[Google developers](https://github.com/orgs/GoogleCloudPlatform/people).

The program will first add all modified files (compared with the last sync) to a
[tarball](https://en.wikipedia.org/wiki/Tar_(computing)) and then, if a secret
was defined, it will encrypt the archive. After that, if AWS was chosen, it will
decide to send it in one shot or use a multipart strategy for larger files. For
now we will follow the AWS suggestion and send multipart when the tarball gets
bigger than 100MB. When using multipart, each part will have 4MB (except for the
last one). The maximum archive size is 40GB (but we can increase this).

Old backups will also be removed automatically, to avoid keeping many files in
the cloud, and consequently saving you some money. Periodically, the tool will
request the remote backups in the cloud to synchronize the local storage.

Some cool features that you will find in this tool:

  * Backup the desired directories periodically;
  * Upload only modified files (small backups parts);
  * Detect ransomware infection (too many modified files);
  * Ignore some files or directories in the backup path;
  * Encrypt backups before sending to the cloud;
  * Automatically download and rebuild backup parts;
  * Old backups are removed periodically to save you some money;
  * List all the versions of a file that was backed up;
  * Smart backup removal, replacing references for incremental backups;
  * Periodic reports sent by e-mail.

## Install

To compile and run the program you will need to download the [Go
compiler](https://golang.org/dl/), set the
[$GOPATH](https://golang.org/doc/code.html#GOPATH), add the `$GOPATH/bin` to
your `$PATH` and run the following command:

```
go get -u github.com/rafaeljusto/toglacier/...
```

If you are thinking that is a good idea to encrypt some sensitive parameters and
want to improve the security, you should replace the numbers of the slices in
the function `passwordKey` of the `encpass_key.go` file for your own random
numbers, or run the python script (inside `internal/config` package) with the
command bellow. Remember to compile the tool again (`go install`).

```
encpass_key_generator.py -w
```

As this program can work like a service/daemon (start command), in this case you
should run it in background. It is a good practice to also add it to your system
startup (you don't want your backup scheduler to stop working after a reboot).

## Usage

The program will work with environment variables or/and with a YAML
configuration file. You can find the configuration file example on
`cmd/toglacier/toglacier.yml`, for the environment variables check bellow:

| Environment Variable                    | Description                             |
| --------------------------------------- | --------------------------------------- |
| TOGLACIER_AWS_ACCOUNT_ID                | AWS account ID                          |
| TOGLACIER_AWS_ACCESS_KEY_ID             | AWS access key ID                       |
| TOGLACIER_AWS_SECRET_ACCESS_KEY         | AWS secret access key                   |
| TOGLACIER_AWS_REGION                    | AWS region                              |
| TOGLACIER_AWS_VAULT_NAME                | AWS vault name                          |
| TOGLACIER_GCS_PROJECT                   | GCS project name                        |
| TOGLACIER_GCS_BUCKET                    | GCS bucket name                         |
| TOGLACIER_GCS_ACCOUNT_FILE              | GCS account file                        |
| TOGLACIER_PATHS                         | Paths to backup (separated by comma)    |
| TOGLACIER_DB_TYPE                       | Local backup storage strategy           |
| TOGLACIER_DB_FILE                       | Path where we keep track of the backups |
| TOGLACIER_LOG_FILE                      | File where all events are written       |
| TOGLACIER_LOG_LEVEL                     | Verbosity of the logger                 |
| TOGLACIER_KEEP_BACKUPS                  | Number of backups to keep (default 10)  |
| TOGLACIER_BACKUP_SECRET                 | Encrypt backups with this secret        |
| TOGLACIER_MODIFY_TOLERANCE              | Maximum percentage of modified files    |
| TOGLACIER_IGNORE_PATTERNS               | Regexps to ignore files in backup paths |
| TOGLACIER_SCHEDULER_BACKUP              | Backup synchronization periodicity      |
| TOGLACIER_SCHEDULER_REMOVE_OLD_BACKUPS  | Remove old backups periodicity          |
| TOGLACIER_SCHEDULER_LIST_REMOTE_BACKUPS | List remote backups periodicity         |
| TOGLACIER_SCHEDULER_SEND_REPORT         | Send report periodicity                 |
| TOGLACIER_EMAIL_SERVER                  | SMTP server address                     |
| TOGLACIER_EMAIL_PORT                    | SMTP server port                        |
| TOGLACIER_EMAIL_USERNAME                | Username for e-mail authentication      |
| TOGLACIER_EMAIL_PASSWORD                | Password for e-mail authentication      |
| TOGLACIER_EMAIL_FROM                    | E-mail used when sending the reports    |
| TOGLACIER_EMAIL_TO                      | List of e-mails to send the report to   |
| TOGLACIER_EMAIL_FORMAT                  | E-mail content format (html or plain)   |

Amazon cloud credentials can be retrieved via AWS Console (`My Security
Credentials` and `Glacier Service`). You will find your AWS region
identification
[here](http://docs.aws.amazon.com/general/latest/gr/rande.html#glacier_region).
For Google Cloud Storage credentials, check the [Service Account
Keys](https://console.developers.google.com/permissions/serviceaccounts). If you
chose Google Cloud Storage, you will need to create the
[project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
and the [bucket](https://cloud.google.com/storage/docs/creating-buckets)
manually.

By default the tool prints everything on the standard output. If you want to
redirect it to a log file, you can define the location of the file with the
`TOGLACIER_LOG_FILE`. Even with the output redirection, the messages are still
written in the standard output. You can define the verbosity using the
`TOGLACIER_LOG_LEVEL` parameter, that can have the values `debug`, `info`,
`warning`, `error`, `fatal` or `panic`. By default the `error` log level is
used.

There are some commands in the tool to manage the backups:

  * **sync**: execute the backup task now
  * **get**: retrieve a backup from AWS Glacier service
  * **list or ls**: list the current backups in the local storage or remotely
  * **remove or rm**: remove a backup from AWS Glacier service
  * **start**: initialize the scheduler (will block forever)
  * **report**: test report notification
  * **encrypt or enc**: encrypt a password or secret to improve security

You can improve the security by encrypting the values (use encrypt command) of
the variables `TOGLACIER_AWS_ACCOUNT_ID`, `TOGLACIER_AWS_ACCESS_KEY_ID`,
`TOGLACIER_AWS_SECRET_ACCESS_KEY`, `TOGLACIER_BACKUP_SECRET` and
`TOGLACIER_EMAIL_PASSWORD`, or the respective variables in the configuration
file. The tool will detect an encrypted value when it starts with the label
`encrypted:`.

For keeping track of the backups locally you can choose `boltdb`
([BoltDB](https://github.com/boltdb/bolt)) or `auditfile` in the
`TOGLACIER_DB_TYPE` variable. By default `boltdb` is used. If you choose the
audit file, as it is a human readable and a technology free solution, the format
is defined bellow. It's a good idea to periodically copy the audit file or the
BoltDB file somewhere else, so if you lose your server you can recover the files
faster from the cloud (don't need to wait for the inventory). If you change your
mind later about what local storage format you want, you can use the
`toglacier-storage` program to convert it. Just remember that `boltdb` format
stores more information than the `auditfile` format.

    [datetime] [vaultName] [archiveID] [checksum] [size] [location]

The `[location]` in the audit file could have the value `aws` or `gcs` depending
on the cloud service used to store the backup.

When running the scheduler (start command), the tool will perform the actions
bellow in the periodicity defined in the configuration file. If not informed
default values are used.

  * backup the files and folders;
  * remove old backups (save storage and money);
  * synchronize the local storage;
  * report all the scheduler occurrences by e-mail.

A shell script that could help you running the program in Unix environments
(using AWS):

```shell
#!/bin/bash

TOGLACIER_AWS_ACCOUNT_ID="encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==" \
TOGLACIER_AWS_ACCESS_KEY_ID="encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ" \
TOGLACIER_AWS_SECRET_ACCESS_KEY="encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=" \
TOGLACIER_AWS_REGION="us-east-1" \
TOGLACIER_AWS_VAULT_NAME="backup" \
TOGLACIER_PATHS="/usr/local/important-files-1,/usr/local/important-files-2" \
TOGLACIER_DB_TYPE="boltdb" \
TOGLACIER_DB_FILE="/var/log/toglacier/toglacier.db" \
TOGLACIER_LOG_FILE="/var/log/toglacier/toglacier.log" \
TOGLACIER_LOG_LEVEL="error" \
TOGLACIER_KEEP_BACKUPS="10" \
TOGLACIER_CLOUD="aws" \
TOGLACIER_BACKUP_SECRET="encrypted:/lFK9sxAXAL8CuM1GYwGsdj4UJQYEQ==" \
TOGLACIER_MODIFY_TOLERANCE="90%" \
TOGLACIER_IGNORE_PATTERNS="^.*\~\$.*$" \
TOGLACIER_SCHEDULER_BACKUP="0 0 0 * * *" \
TOGLACIER_SCHEDULER_REMOVE_OLD_BACKUPS="0 0 1 * * FRI" \
TOGLACIER_SCHEDULER_LIST_REMOTE_BACKUPS="0 0 12 1 * *" \
TOGLACIER_SCHEDULER_SEND_REPORT="0 0 6 * * FRI" \
TOGLACIER_EMAIL_SERVER="smtp.example.com" \
TOGLACIER_EMAIL_PORT="587" \
TOGLACIER_EMAIL_USERNAME="user@example.com" \
TOGLACIER_EMAIL_PASSWORD="encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==" \
TOGLACIER_EMAIL_FROM="user@example.com" \
TOGLACIER_EMAIL_TO="report1@example.com,report2@example.com" \
TOGLACIER_EMAIL_FORMAT="html" \
toglacier $@
```

With that you can just run the following command to start the scheduler:

```
./toglacier.sh start
```

Just remember to give the write permissions to where the stdout/stderr and audit
files are going to be written (`/var/log/toglacier`).

## Deployment

For developers that want to build a package, we already have 2 scripts to make
your life easier. As Go can do some cross-compilation, you can build the
desired package from any OS or architecture.

### Debian

To build a Debian package you will need the [Effing Package
Management](https://github.com/jordansissel/fpm/wiki) tool. Then just run the
script with the desired version and release of the program:

    ./package-deb.sh <version>-<release>

### FreeBSD

You can also build a package for the FreeBSD
[pkgng](https://wiki.freebsd.org/pkgng) repository. No external tools needed
here to build the package.

    ./package-txz.sh <version>-<release>

### Windows

To make your life easier you can use the tool [NSSM](http://nssm.cc) to build a
Windows service to run the toglacier tool in background. The following commands
would install the service:

```
c:\> nssm.exe install toglacier

c:\> nssm.exe start toglacier
```