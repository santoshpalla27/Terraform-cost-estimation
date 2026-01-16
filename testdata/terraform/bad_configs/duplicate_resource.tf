# Bad Config: Duplicate resource name

resource "aws_instance" "duplicate" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

# This would be a Terraform error - duplicate resource name
resource "aws_instance" "duplicate" {
  ami           = "ami-87654321"
  instance_type = "t3.small"
}
