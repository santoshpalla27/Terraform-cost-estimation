# Bad Config: Circular dependency

resource "aws_security_group" "circular_a" {
  name   = "circular-a"
  vpc_id = "vpc-12345678"

  ingress {
    from_port       = 80
    to_port         = 80
    protocol        = "tcp"
    security_groups = [aws_security_group.circular_b.id]
  }
}

resource "aws_security_group" "circular_b" {
  name   = "circular-b"
  vpc_id = "vpc-12345678"

  ingress {
    from_port       = 443
    to_port         = 443
    protocol        = "tcp"
    security_groups = [aws_security_group.circular_a.id]
  }
}

# This will cause a Terraform cycle error
