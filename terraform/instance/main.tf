terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "viafoura-clouddesktop-tfstate"
    region         = "us-east-1"
    dynamodb_table = "viafoura-clouddesktop-tfstate-lock"
    encrypt        = true
    # key is provided at runtime via: terraform init -backend-config="key=developers/<name>/terraform.tfstate"
    # The clouddesktop CLI passes the key dynamically during initialization to isolate each developer's state
  }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project   = "clouddesktop"
      ManagedBy = "terraform"
    }
  }
}
