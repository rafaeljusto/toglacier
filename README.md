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

## Usage

For now this program will only work with environment variables. So you need to
set the following before running the program:

  | Environment Variable  | Description                             |
  | --------------------- | --------------------------------------- |
  | AWS_ACCOUNT_ID        | AWS account ID                          |
  | AWS_ACCESS_KEY_ID     | AWS access key ID                       |
  | AWS_SECRET_ACCESS_KEY | AWS secret access key                   |
  | AWS_REGION            | AWS region                              |
  | AWS_VAULT_NAME        | AWS vault name                          |
  | TOGLACIER_PATH        | Path to backup                          |
  | TOGLACIER_AUDIT       | Path where we keep track of the backups |

Most part of them you can retrieve via AWS Console (`My Security Credentials`
and `Glacier Service`). You will find your AWS region identification
[here](http://docs.aws.amazon.com/general/latest/gr/rande.html#glacier_region).

The audit file that keeps track of all backups has the following format:

    [datetime] [location] [checksum]

**This software is still under development.**
