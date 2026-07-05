resource "aws_iam_role" "developer_instance" {
  name        = "clouddesktop-developer-instance"
  description = "IAM role for CloudDesktop developer EC2 instances"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  tags = {
    Name = "clouddesktop-developer-instance"
  }
}

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.developer_instance.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy_attachment" "cloudwatch" {
  role       = aws_iam_role.developer_instance.name
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
}

resource "aws_iam_policy" "ecr_read" {
  name        = "clouddesktop-ecr-read"
  description = "Allow CloudDesktop developer instances to pull from ECR"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchGetImage",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchCheckLayerAvailability",
          "ecr:DescribeRepositories",
          "ecr:ListImages"
        ]
        Resource = "arn:aws:ecr:us-east-1:${data.aws_caller_identity.current.account_id}:repository/*"
      }
    ]
  })

  tags = {
    Name = "clouddesktop-ecr-read"
  }
}

resource "aws_iam_role_policy_attachment" "ecr_read" {
  role       = aws_iam_role.developer_instance.name
  policy_arn = aws_iam_policy.ecr_read.arn
}

resource "aws_iam_policy" "s3_maven_read" {
  name        = "clouddesktop-s3-maven-read"
  description = "Allow CloudDesktop developer instances to read from the vf-maven S3 Maven repository"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::vf-maven",
          "arn:aws:s3:::vf-maven/*"
        ]
      }
    ]
  })

  tags = {
    Name = "clouddesktop-s3-maven-read"
  }
}

resource "aws_iam_role_policy_attachment" "s3_maven_read" {
  role       = aws_iam_role.developer_instance.name
  policy_arn = aws_iam_policy.s3_maven_read.arn
}

resource "aws_iam_policy" "codeartifact_read" {
  name        = "clouddesktop-codeartifact-read"
  description = "Allow CloudDesktop developer instances to read from CodeArtifact Maven repositories"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "codeartifact:GetAuthorizationToken",
          "codeartifact:GetRepositoryEndpoint",
          "codeartifact:ReadFromRepository"
        ]
        Resource = [
          "arn:aws:codeartifact:us-east-1:${data.aws_caller_identity.current.account_id}:domain/viafoura",
          "arn:aws:codeartifact:us-east-1:${data.aws_caller_identity.current.account_id}:repository/viafoura/*"
        ]
      },
      {
        Effect   = "Allow"
        Action   = "sts:GetServiceBearerToken"
        Resource = "*"
        Condition = {
          StringEquals = {
            "sts:AWSServiceName" = "codeartifact.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = {
    Name = "clouddesktop-codeartifact-read"
  }
}

resource "aws_iam_role_policy_attachment" "codeartifact_read" {
  role       = aws_iam_role.developer_instance.name
  policy_arn = aws_iam_policy.codeartifact_read.arn
}

resource "aws_iam_policy" "s3_developer_bucket" {
  name        = "clouddesktop-s3-developer-bucket"
  description = "Allow CloudDesktop instances to read/write their developer S3 bucket (Mountpoint for S3)"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket",
          "s3:GetBucketLocation"
        ]
        Resource = [
          "arn:aws:s3:::clouddesktop-*",
          "arn:aws:s3:::clouddesktop-*/*"
        ]
      }
    ]
  })

  tags = {
    Name = "clouddesktop-s3-developer-bucket"
  }
}

resource "aws_iam_role_policy_attachment" "s3_developer_bucket" {
  role       = aws_iam_role.developer_instance.name
  policy_arn = aws_iam_policy.s3_developer_bucket.arn
}

resource "aws_iam_instance_profile" "developer_instance" {
  name = "clouddesktop-developer-instance"
  role = aws_iam_role.developer_instance.name

  tags = {
    Name = "clouddesktop-developer-instance"
  }
}
