output "instance_id" {
  description = "EC2 instance ID for the developer environment"
  value       = aws_instance.developer.id
}

output "private_ip" {
  description = "Private IP address of the developer instance"
  value       = aws_instance.developer.private_ip
}

output "availability_zone" {
  description = "Availability zone where the instance is running"
  value       = aws_instance.developer.availability_zone
}

output "key_pair_name" {
  description = "Name of the SSH key pair used for the instance"
  value       = aws_key_pair.developer.key_name
}
