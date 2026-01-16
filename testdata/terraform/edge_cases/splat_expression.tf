# Edge Case: Splat expression and resource references

variable "instance_count" {
  type    = number
  default = 3
}

resource "aws_instance" "cluster" {
  count = var.instance_count

  ami           = "ami-12345678"
  instance_type = "t3.micro"

  tags = {
    Name = "cluster-${count.index}"
  }
}

# Using splat expression
resource "aws_lb_target_group_attachment" "cluster" {
  count = var.instance_count

  target_group_arn = "arn:aws:elasticloadbalancing:us-east-1:123456789:targetgroup/main/abc123"
  target_id        = aws_instance.cluster[count.index].id
  port             = 80
}

# Using splat to get all IDs
output "instance_ids" {
  value = aws_instance.cluster[*].id
}

output "private_ips" {
  value = aws_instance.cluster[*].private_ip
}
