variable "vpc_id" {
  description = "ID of the existing VPC where developer instances will be deployed"
  type        = string
}

variable "subnet_id" {
  description = "ID of the subnet where developer instances will be launched"
  type        = string
}

variable "tfstate_bucket_name" {
  description = "Name of the S3 bucket for Terraform state storage"
  type        = string
  default     = "clouddesktop-tfstate"
}

variable "tfstate_lock_table_name" {
  description = "Name of the DynamoDB table for Terraform state locking"
  type        = string
  default     = "clouddesktop-tfstate-lock"
}
