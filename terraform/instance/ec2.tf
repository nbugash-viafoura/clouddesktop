data "aws_ssm_parameter" "vpc_id" {
  name = "/clouddesktop/shared/vpc_id"
}

data "aws_subnets" "clouddesktop" {
  filter {
    name   = "vpc-id"
    values = [data.aws_ssm_parameter.vpc_id.value]
  }
}

data "aws_ssm_parameter" "security_group_id" {
  name = "/clouddesktop/shared/security_group_id"
}

data "aws_ssm_parameter" "instance_profile_name" {
  name = "/clouddesktop/shared/instance_profile_name"
}

data "aws_ami" "ubuntu_22_04" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_key_pair" "developer" {
  key_name   = "clouddesktop-${var.developer_name}"
  public_key = var.ssh_public_key

  tags = {
    Name      = "clouddesktop-${var.developer_name}"
    Developer = var.developer_name
  }
}

resource "aws_instance" "developer" {
  ami                     = data.aws_ami.ubuntu_22_04.id
  instance_type           = var.instance_type
  subnet_id               = data.aws_subnets.clouddesktop.ids[0]
  vpc_security_group_ids  = [data.aws_ssm_parameter.security_group_id.value]
  key_name                = aws_key_pair.developer.key_name
  iam_instance_profile    = data.aws_ssm_parameter.instance_profile_name.value
  user_data               = file("${path.module}/../../scripts/bootstrap-system.sh")
  user_data_replace_on_change = false
  disable_api_termination = false

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 100
    throughput            = 300
    iops                  = 3000
    delete_on_termination = true
    encrypted             = true

    tags = {
      Name      = "clouddesktop-${var.developer_name}-root"
      Developer = var.developer_name
    }
  }

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    instance_metadata_tags      = "enabled"
  }

  tags = {
    Name      = "clouddesktop-${var.developer_name}"
    Developer = var.developer_name
  }

  lifecycle {
    ignore_changes = [user_data]
  }
}
