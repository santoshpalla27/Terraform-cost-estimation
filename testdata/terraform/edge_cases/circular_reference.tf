# Edge Case: Circular reference between resources

resource "aws_security_group" "sg_a" {
  name        = "sg-a"
  description = "Security group A"
  vpc_id      = "vpc-12345678"

  ingress {
    from_port       = 80
    to_port         = 80
    protocol        = "tcp"
    security_groups = [aws_security_group.sg_b.id]
  }
}

resource "aws_security_group" "sg_b" {
  name        = "sg-b"
  description = "Security group B"
  vpc_id      = "vpc-12345678"

  ingress {
    from_port       = 443
    to_port         = 443
    protocol        = "tcp"
    security_groups = [aws_security_group.sg_a.id]
  }
}
