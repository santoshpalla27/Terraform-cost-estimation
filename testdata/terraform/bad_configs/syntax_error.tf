# Bad Config: Syntax error

resource "aws_instance" "broken" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"

  tags = {
    Name = "broken"
  # Missing closing brace
}
