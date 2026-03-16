output "developer_instance_sg_id" {
  description = "Security group ID for developer instances"
  value       = aws_security_group.developer_instance.id
}

output "instance_profile_name" {
  description = "IAM instance profile name for developer instances"
  value       = aws_iam_instance_profile.developer_instance.name
}

output "tfstate_bucket_name" {
  description = "S3 bucket name for Terraform state storage"
  value       = aws_s3_bucket.tfstate.bucket
}
