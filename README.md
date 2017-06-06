[![GoDoc](https://godoc.org/github.com/rafaeljusto/toglacier?status.png)](https://godoc.org/github.com/rafaeljusto/toglacier)
[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/rafaeljusto/toglacier/master/LICENSE)
[![Build Status](https://travis-ci.org/rafaeljusto/toglacier.png?branch=master)](https://travis-ci.org/rafaeljusto/toglacier)
[![Coverage Status](https://coveralls.io/repos/github/rafaeljusto/toglacier/badge.svg?branch=master)](https://coveralls.io/github/rafaeljusto/toglacier?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/rafaeljusto/toglacier)](https://goreportcard.com/report/github.com/rafaeljusto/toglacier)
[![codebeat badge](https://codebeat.co/badges/f772925e-c2b3-4631-9f19-83cca1dc3a4b)](https://codebeat.co/projects/github-com-rafaeljusto-toglacier)

![toglacier](https://raw.githubusercontent.com/rafaeljusto/toglacier/master/toglacier.png)

# toglacier
Send data to Amazon Glacier service periodically.

## What?

Have you ever thought that your server could have some backup in the cloud to
mitigate some crazy [ransomware](https://en.wikipedia.org/wiki/Ransomware)
infection? Great! Here is a peace of software to help you do that, sending your
data periodically to [Amazon Glacier](https://aws.amazon.com/glacier/). It uses
the [AWS SDK](https://aws.amazon.com/sdk-for-go/) behind the scenes, all honors
go to the [Amazon developers](https://github.com/orgs/aws/people).

The program will first add all modified files (compared with the last sync) to a
[tarball](https://en.wikipedia.org/wiki/Tar_(computing)) and then, if a secret
was defined, it will encrypt the archive. After that it will decide to send it
in one shot or use a multipart strategy for larger files. For now we will follow
the AWS suggestion and send multipart when the tarball gets bigger than 100MB.
When using multipart, each part will have 4MB (except for the last one). The
maximum archive size is 40GB (but we can increase this).

Old backups will also be removed automatically, to avoid keeping many files in
AWS Glacier service, and consequently saving you some money. Periodically, the
tool will request the remote backups in AWS to synchronize the local storage.

Some cool features that you will find in this tool:

  * Backup the desired directories periodically;
  * Upload only modified files (small backups parts);
  * Encrypt backups before sending to the cloud;
  * Automatically download and rebuild backup parts;
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
the function `passwordKey` of the `encpass.go` file for your own random numbers.
Remember to compile the tool again (`go install`).

As this program can work like a service/daemon (start command), in this case you
should run it in background. It is a good practice to also add it to your system
startup (you don't want your backup scheduler to stop working after a reboot).

## Usage

The program will work with environment variables or/and with a YAML
configuration file. You can find the configuration file example on
`cmd/toglacier/toglacier.yml`, for the environment variables check bellow:

| Environment Variable             | Description                             |
| -------------------------------- | --------------------------------------- |
| TOGLACIER_AWS_ACCOUNT_ID         | AWS account ID                          |
| TOGLACIER_AWS_ACCESS_KEY_ID      | AWS access key ID                       |
| TOGLACIER_AWS_SECRET_ACCESS_KEY  | AWS secret access key                   |
| TOGLACIER_AWS_REGION             | AWS region                              |
| TOGLACIER_AWS_VAULT_NAME         | AWS vault name                          |
| TOGLACIER_PATHS                  | Paths to backup (separated by comma)    |
| TOGLACIER_DB_TYPE                | Local backup storage strategy           |
| TOGLACIER_DB_FILE                | Path where we keep track of the backups |
| TOGLACIER_LOG_FILE               | File where all events are written       |
| TOGLACIER_LOG_LEVEL              | Verbosity of the logger                 |
| TOGLACIER_KEEP_BACKUPS           | Number of backups to keep (default 10)  |
| TOGLACIER_BACKUP_SECRET          | Encrypt backups with this secret        |
| TOGLACIER_EMAIL_SERVER           | SMTP server address                     |
| TOGLACIER_EMAIL_PORT             | SMTP server port                        |
| TOGLACIER_EMAIL_USERNAME         | Username for e-mail authentication      |
| TOGLACIER_EMAIL_PASSWORD         | Password for e-mail authentication      |
| TOGLACIER_EMAIL_FROM             | E-mail used when sending the reports    |
| TOGLACIER_EMAIL_TO               | List of e-mails to send the report to   |
| TOGLACIER_EMAIL_FORMAT           | E-mail content format (html or plain)   |

Most part of them you can retrieve via AWS Console (`My Security Credentials`
and `Glacier Service`). You will find your AWS region identification
[here](http://docs.aws.amazon.com/general/latest/gr/rande.html#glacier_region).

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
faster from the AWS Glacier (don't need to wait for the inventory).

    [datetime] [vaultName] [archiveID] [checksum] [size]

When running the scheduler (start command), **the tool will backup the files
once a day at midnight**. This information isn't configurable yet (the library
that I'm using for cron tasks isn't so flexible). Also, **old backups are
removed once a week at 1 AM** (yep, not configurable yet). To keep the
consistency, **local storage synchronization will occur once a month at 12 PM**.
A **report will be generated and sent once a week at 6 AM** with all the
scheduler occurrences.

A simple shell script that could help you running the program in Unix
environments:

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
TOGLACIER_BACKUP_SECRET="encrypted:/lFK9sxAXAL8CuM1GYwGsdj4UJQYEQ==" \
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
would install the service (replace the necessary parameters):

```
c:\> nssm.exe install toglacier c:\programs\toglacier.exe start

c:\> nssm.exe set toglacier AppEnvironmentExtra ^
  TOGLACIER_AWS_ACCOUNT_ID=encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w== ^
  TOGLACIER_AWS_ACCESS_KEY_ID=encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ ^
  TOGLACIER_AWS_SECRET_ACCESS_KEY=encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA= ^
  TOGLACIER_AWS_REGION=us-east-1 ^
  TOGLACIER_AWS_VAULT_NAME=backup ^
  TOGLACIER_PATHS=c:\data\important-files-1,c:\data\important-files-2 ^
  TOGLACIER_DB_TYPE=boltdb ^
  TOGLACIER_DB_FILE=c:\log\toglacier\toglacier.db ^
  TOGLACIER_LOG_FILE=c:\log\toglacier\toglacier.log ^
  TOGLACIER_LOG_LEVEL=error ^
  TOGLACIER_KEEP_BACKUPS=10 ^
  TOGLACIER_BACKUP_SECRET=encrypted:/lFK9sxAXAL8CuM1GYwGsdj4UJQYEQ== ^
  TOGLACIER_EMAIL_SERVER=smtp.example.com ^
  TOGLACIER_EMAIL_PORT=587 ^
  TOGLACIER_EMAIL_USERNAME=user@example.com ^
  TOGLACIER_EMAIL_PASSWORD=encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA== ^
  TOGLACIER_EMAIL_FROM=user@example.com ^
  TOGLACIER_EMAIL_TO=report1@example.com,report2@example.com ^
  TOGLACIER_EMAIL_FORMAT=html

c:\> nssm.exe start toglacier
```