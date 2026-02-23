# Use the existing Development VPC in the viafoura-test account.
# VPC: vpc-597ca83c (10.3.0.0/16), Name: "Development"
# Subnet: subnet-d3facefb (10.3.36.0/23), Name: "Services - B", AZ: us-east-1b
data "aws_vpc" "development" {
  id = "vpc-597ca83c"
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
  value = "subnet-d3facefb"

  tags = {
    Name = "clouddesktop-subnet-id"
  }
}
