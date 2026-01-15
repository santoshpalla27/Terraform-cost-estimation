# Basic Test: Simple single resource

resource "aws_instance" "simple" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"

  tags = {
    Name        = "simple-instance"
    Environment = "test"
  }
}
