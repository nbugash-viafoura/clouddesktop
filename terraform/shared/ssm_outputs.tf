resource "aws_ssm_parameter" "security_group_id" {
  name  = "/clouddesktop/shared/security_group_id"
  type  = "String"
  tier  = "Standard"
  value = aws_security_group.developer_instance.id

  tags = {
    Name = "clouddesktop-security-group-id"
  }
}

resource "aws_ssm_parameter" "instance_profile_name" {
  name  = "/clouddesktop/shared/instance_profile_name"
  type  = "String"
  tier  = "Standard"
  value = aws_iam_instance_profile.developer_instance.name

  tags = {
    Name = "clouddesktop-instance-profile-name"
  }
}
