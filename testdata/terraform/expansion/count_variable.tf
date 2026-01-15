# Expansion Test: count from variable

variable "instance_count" {
  type    = number
  default = 3
}

resource "aws_instance" "counted" {
  count         = var.instance_count
  ami           = "ami-12345678"
  instance_type = "t3.micro"

  tags = {
    Name = "instance-${count.index}"
  }
}
