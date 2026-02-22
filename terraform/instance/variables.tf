variable "developer_name" {
  description = "Name of the developer - used in resource names, tags, and state key"
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.developer_name))
    error_message = "Developer name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "instance_type" {
  description = "EC2 instance type for the developer environment"
  type        = string
  default     = "m7i.xlarge"

  validation {
    condition     = can(regex("^[a-z][0-9][a-z0-9]*\\.[0-9]*[a-z]+$", var.instance_type))
    error_message = "Instance type must be a valid EC2 instance type format."
  }
}

variable "ssh_public_key" {
  description = "ED25519 SSH public key content (not file path) for the developer"
  type        = string
  sensitive   = true

  validation {
    condition     = can(regex("^ssh-ed25519 AAAA[0-9A-Za-z+/]+[=]{0,3}", var.ssh_public_key))
    error_message = "SSH public key must be a valid ED25519 public key."
  }
}

variable "region" {
  description = "AWS region for the developer environment"
  type        = string
  default     = "us-east-1"

  validation {
    condition     = can(regex("^[a-z]{2}-[a-z]+-[0-9]{1}$", var.region))
    error_message = "Region must be a valid AWS region format."
  }
}
