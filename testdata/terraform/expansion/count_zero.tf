# Expansion Test: count=0 produces nothing

resource "aws_instance" "zero" {
  count         = 0
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}
