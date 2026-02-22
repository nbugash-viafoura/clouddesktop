# Dedicated VPC for CloudDesktop instances
resource "aws_vpc" "clouddesktop" {
  cidr_block           = "10.200.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "clouddesktop"
  }
}

# Internet Gateway for egress
resource "aws_internet_gateway" "clouddesktop" {
  vpc_id = aws_vpc.clouddesktop.id

  tags = {
    Name = "clouddesktop"
  }
}

# Subnet for CloudDesktop instances
resource "aws_subnet" "clouddesktop" {
  vpc_id                  = aws_vpc.clouddesktop.id
  cidr_block              = "10.200.1.0/24"
  availability_zone       = "us-east-1a"
  map_public_ip_on_launch = false

  tags = {
    Name = "clouddesktop"
  }
}

# Route table for outbound internet access
resource "aws_route_table" "clouddesktop" {
  vpc_id = aws_vpc.clouddesktop.id

  route {
    cidr_block      = "0.0.0.0/0"
    gateway_id      = aws_internet_gateway.clouddesktop.id
  }

  tags = {
    Name = "clouddesktop"
  }
}

# Associate subnet with route table
resource "aws_route_table_association" "clouddesktop" {
  subnet_id      = aws_subnet.clouddesktop.id
  route_table_id = aws_route_table.clouddesktop.id
}

# Store VPC and subnet ID in SSM parameters for developer instances to reference
resource "aws_ssm_parameter" "vpc_id" {
  name  = "/clouddesktop/shared/vpc_id"
  type  = "String"
  value = aws_vpc.clouddesktop.id

  tags = {
    Name = "clouddesktop-vpc-id"
  }
}

resource "aws_ssm_parameter" "subnet_id" {
  name  = "/clouddesktop/shared/subnet_id"
  type  = "String"
  value = aws_subnet.clouddesktop.id

  tags = {
    Name = "clouddesktop-subnet-id"
  }
}

