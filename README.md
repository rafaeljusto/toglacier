![toglacier](https://raw.githubusercontent.com/rafaeljusto/toglacier/master/toglacier.png)

# toglacier
Send data to Amazon Glacier service periodically.

## What?

Have you ever thought that your server could have some backup in the cloud to
mitigate some crazy [ransomware](https://en.wikipedia.org/wiki/Ransomware)
infection? Great! Here is a peace of software to help you do that, sending your
data periodically to [Amazon Glacier](https://aws.amazon.com/glacier/). It uses
the [AWS SDK](https://aws.amazon.com/sdk-for-go/) behind the scenes, so this
program is really only a dummy layer to make your life easier, all honors go to
the [Amazon developers](https://github.com/orgs/aws/people).

The program will first add all files to a
[tarball](https://en.wikipedia.org/wiki/Tar_(computing)) and then decide to send
it in one shot or use a multipart strategy for larger files. For now we will
follow the AWS suggestion and send multipart when the tarball gets bigger than
100MB. When using multipart, each part will have 4MB (except for the last one).
The maximum archive size is 40GB (but we can increase this).

Old backups will also be removed automatically, to avoid keeping many files in
AWS Glacier service, and consequently saving you some money.

## Install

To compile and run the program you will need to download the [Go
compiler](https://golang.org/dl/), set the
[$GOPATH](https://golang.org/doc/code.html#GOPATH), add the `$GOPATH/bin` to
your `$PATH` and run the following command:

```
go get -u github.com/rafaeljusto/toglacier
```

If you are thinking that is a good idea to encrypt some sensitive tool
parameters and want to improve the security, is a good idea to replace the
numbers of the slices in the function `passwordKey` of the `encpass.go` file for
your random numbers. Remember to compile the tool again (`go install`).

As this program can work like a service/daemon (start command), in this case you
should run it in background. It is a good practice to also add it to your system
startup (you don't want your backup scheduler to stop working after a reboot).

## Usage

For now this program will only work with environment variables. So you need to
set the following before running the program:

| Environment Variable   | Description                             |
| ---------------------- | --------------------------------------- |
| AWS_ACCOUNT_ID         | AWS account ID                          |
| AWS_ACCESS_KEY_ID      | AWS access key ID                       |
| AWS_SECRET_ACCESS_KEY  | AWS secret access key                   |
| AWS_REGION             | AWS region                              |
| AWS_VAULT_NAME         | AWS vault name                          |
| TOGLACIER_PATH         | Paths to backup (separated by comma)    |
| TOGLACIER_AUDIT        | Path where we keep track of the backups |
| TOGLACIER_KEEP_BACKUPS | Number of backups to keep (default 10)  |

Most part of them you can retrieve via AWS Console (`My Security Credentials`
and `Glacier Service`). You will find your AWS region identification
[here](http://docs.aws.amazon.com/general/latest/gr/rande.html#glacier_region).

There are some commands in the tool to manage the backups:

  * **sync**: execute the backup task now
  * **get**: retrive a backup from AWS Glacier service
  * **list or ls**: list the current backups using a local audit file or remotly
  * **remove or rm**: remove a backup from AWS Glacier service
  * **start**: initialize the scheduler (will block forever)
  * **encrypt or enc**: encrypt a password or secret to improve security

You can improve the security by encrypting the values (use encrypt command) of
the variables `AWS_ACCOUNT_ID`, `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.
The tool will detect an encrypted value when it starts with the label
`encrypted:`.

The audit file that keeps track of all backups has the format bellow. It's a
good idea to periodically copy this audit file somewhere else, so if you lose
your server you can recorver the files faster from the AWS Glacier (don't need
to wait for the iventory).

    [datetime] [vaultName] [archiveID] [checksum]

When running the scheduler (start command), **the tool will backup the files
once a day at midnight**. This information isn't configurable yet (the library
that I'm using for cron tasks isn't so flexible). Also, **old backups are
removed once a week at 1 AM** (yep, not configurable yet).

A simple shell script that could help you running the program in Unix
environments:

```shell
#!/bin/sh

AWS_ACCOUNT_ID="encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==" \
AWS_ACCESS_KEY_ID="encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ" \
AWS_SECRET_ACCESS_KEY="encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=" \
AWS_REGION="us-east-1" \
AWS_VAULT_NAME="backup" \
TOGLACIER_PATH="/usr/local/important-files-1,/usr/local/important-files-2" \
TOGLACIER_AUDIT="/var/log/toglacier/audit.log" \
TOGLACIER_KEEP_BACKUPS="10" \
toglacier $@ &>> /var/log/toglacier/error.log
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
