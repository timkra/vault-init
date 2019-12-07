# vault-init

Originally forked from [sethvargo/vault-init](https://github.com/sethvargo/vault-init) and ported to AWS.

The `vault-init` service automates the process of [initializing](https://www.vaultproject.io/docs/commands/operator/init.html) HashiCorp [Vault](https://www.vaultproject.io/) instances running on [Amazon Web Services](https://aws.amazon.com/). Unlike the original this service does not offer unsealing. The service should be used alongside Vault [auto-unseal](https://www.vaultproject.io/docs/concepts/seal.html#auto-unseal).

After `vault-init` initializes a Vault server it stores recovery keys and root token to user defined [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/) Secrets.
The Secrets can be encrypted using [AWS KMS Key](https://aws.amazon.com/kms). See [How AWS Secrets Manager Uses AWS KMS](https://docs.aws.amazon.com/kms/latest/developerguide/services-secrets-manager.html) for more information. When using this feature make sure the service has permission to the Key.

The original service stores the keys and token in a Bucket. The decision to store them in Secrets Manager was made, because Terraforms data source [aws_s3_bucket_object](https://www.terraform.io/docs/providers/aws/d/s3_bucket_object.html) can only use files that are plain text.

However using the data source [aws_secretsmanager_secret](https://www.terraform.io/docs/providers/aws/d/secretsmanager_secret.html) it is possible retrieve the root token, configure Terraforms [Vault Provider](https://www.terraform.io/docs/providers/vault/index.html) and create modules to configure Vault.

## Usage

The `vault-init` service is designed to be run alongside a Vault server and
communicate over local host.

You can download the code and compile the binary with Go, or create a Docker container.

To use this as part of a Kubernetes Deployment:

```yaml
containers:
- name: vault-init
  image: {{repository}}/vault-init:{{tag}}
  imagePullPolicy: Always
  env:
  - name: ROOT_TOKEN_SECRET_ID
    value: vault-root-token
  - name: "RECOVERY_KEYS_SECRET_ID"
    value: vault-recovery-keys
```

## Configuration

The vault-init service supports the following environment variables for configuration:

- `CHECK_INTERVAL` - The time duration between Vault health checks. ("10s")

- `VAULT_STORED_SHARES` - Number of shares to store on KMS. - Default: 1

- `VAULT_RECOVERY_SHARES` - Number of recovery shares to generate. - Default: 1

- `VAULT_RECOVERY_THRESHOLD` - Number of recovery shares needed to unseal. - Default: 1

- `ROOT_TOKEN_SECRET_ID` - The secret where Vaults root token is stored.

- `RECOVERY_KEYS_SECRET_ID` - The secret where Vaults recovery keys are stored.

### Example Values

```text
CHECK_INTERVAL="30s"
ROOT_TOKEN_SECRET_ID="vault-root-token"
RECOVERY_KEYS_SECRET_ID="vault-recovery-keys"
```

### IAM &amp; Permissions

The `vault-init` service uses the official Amazon Web Service Golang SDK. This means
it supports the common ways of [providing credentials to AWS](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).

To use this service, the IAM Role or IAM User must have permissions on the KMS Key, as well as permission on Secrets Manager.
