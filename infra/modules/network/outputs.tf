output "vpc_id" {
  value = aws_vpc.this.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  value = aws_subnet.private[*].id
}

output "alb_sg_id" {
  value = aws_security_group.alb.id
}

output "tasks_sg_id" {
  value = aws_security_group.tasks.id
}

output "data_sg_id" {
  value = aws_security_group.data.id
}
