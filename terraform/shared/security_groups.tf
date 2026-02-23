resource "aws_security_group" "developer_instance" {
  name        = "clouddesktop-developer-instance"
  description = "Security group for CloudDesktop developer EC2 instances"
  vpc_id      = data.aws_vpc.development.id

  egress {
    description = "Allow all outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "clouddesktop-developer-instance"
  }
}
