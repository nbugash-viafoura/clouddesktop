terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Bootstrap note: on first apply, comment out the S3 backend block and use local state.
  # After apply completes and the S3 bucket exists, uncomment the S3 backend and run
  # terraform init -migrate-state to move state to S3.
  backend "s3" {
    bucket         = "viafoura-clouddesktop-tfstate"
    key            = "shared/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "viafoura-clouddesktop-tfstate-lock"
    encrypt        = true
  }
}

provider "aws" {
  region = "us-east-1"

  default_tags {
    tags = {
      ManagedBy = "terraform"
      Project   = "clouddesktop"
    }
  }
}

data "aws_caller_identity" "current" {}
