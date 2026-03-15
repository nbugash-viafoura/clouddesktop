# Use an existing VPC in the target AWS account.
# Update the vpc_id and subnet_id variables in variables.tf to match your environment.
data "aws_vpc" "development" {
  id = var.vpc_id
}

# Store VPC and subnet ID in SSM parameters for developer instances to reference
resource "aws_ssm_parameter" "vpc_id" {
  name  = "/clouddesktop/shared/vpc_id"
  type  = "String"
  value = data.aws_vpc.development.id

  tags = {
    Name = "clouddesktop-vpc-id"
  }
}

resource "aws_ssm_parameter" "subnet_id" {
  name  = "/clouddesktop/shared/subnet_id"
  type  = "String"
  value = var.subnet_id

  tags = {
    Name = "clouddesktop-subnet-id"
  }
}
