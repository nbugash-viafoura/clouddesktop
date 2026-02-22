# CloudDesktop Shared Infrastructure (Tier 1)

This Terraform root creates the shared infrastructure for Viafoura's CloudDesktop remote developer environment system.

## What This Creates

- **VPC** with private subnets across two AZs (us-east-1a, us-east-1b)
- **VPC Endpoints** for ECR, S3, and SSM (enabling private, internet-free access)
- **Security Groups** for developer instances and VPC endpoints
- **IAM Role and Instance Profile** for developer EC2 instances
- **S3 Backend** for Terraform state management (shared across all CloudDesktop roots)
- **DynamoDB Table** for Terraform state locking
- **SSM Parameters** exposing shared infrastructure values to instance-level Terraform

## Prerequisites

- AWS CLI configured with `test-terraform` profile
- Terraform >= 1.6 installed
- Access to AWS account `218894879100` (test environment)

## Bootstrap Process

Because this root creates its own S3 backend, the initial apply requires a two-step process:

### Step 1: Initial Apply with Local State

1. Edit `main.tf` and **comment out** the entire S3 backend block (lines 14-21).

2. Initialize and apply with local state:

```bash
cd /Users/nonicobugash/Development/Viafoura/toolings/clouddesktop/terraform/shared
terraform init
terraform plan
terraform apply
```

This creates the S3 bucket `viafoura-clouddesktop-tfstate` and DynamoDB table `viafoura-clouddesktop-tfstate-lock`.

### Step 2: Migrate to S3 Backend

1. Edit `main.tf` and **uncomment** the S3 backend block.

2. Re-initialize and migrate state:

```bash
terraform init -migrate-state
```

When prompted, confirm the migration. Terraform will copy the local state file to S3.

3. Verify state is now in S3:

```bash
aws s3 ls s3://viafoura-clouddesktop-tfstate/shared/ --profile test-terraform
```

You should see `terraform.tfstate`.

4. Delete the local state files (they are now safely in S3):

```bash
rm -f terraform.tfstate terraform.tfstate.backup
```

## Usage After Bootstrap

Once bootstrapped, all future applies use the S3 backend automatically:

```bash
terraform plan
terraform apply
```

## Outputs

After apply, the following values are available:

- `vpc_id` — VPC ID
- `subnet_id_a` — Private subnet in us-east-1a
- `subnet_id_b` — Private subnet in us-east-1b
- `developer_instance_sg_id` — Security group for developer instances
- `instance_profile_name` — IAM instance profile name
- `tfstate_bucket_name` — S3 bucket for state storage

These values are also written to SSM Parameter Store at `/clouddesktop/shared/*` for consumption by instance-level Terraform.

## Architecture Notes

- **No public subnets** — instances are 100% private, accessed via SSM Session Manager
- **Zero ingress rules** on developer instance security group — SSM is the only access path
- **Gateway endpoint for S3** — cost-free, used for ECR image layer storage
- **Interface endpoints for ECR/SSM** — enable private connectivity without internet gateway routing
- **State locking enabled** — DynamoDB table prevents concurrent modifications
- **State encryption** — S3 bucket uses AES256 server-side encryption

## Important Notes

- This Terraform root is applied **ONCE** by an admin
- It does **NOT** create any EC2 instances
- Developer instances are created separately in `terraform/instance/` using the `test-developers` profile
- The S3 bucket and DynamoDB table have `lifecycle { prevent_destroy = true }` to prevent accidental deletion
