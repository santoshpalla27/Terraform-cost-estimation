# Bad Config: Invalid for_each (not map or set)

variable "instance_count" {
  type    = number
  default = 3
}

# for_each requires map or set, not number
resource "aws_instance" "invalid_for_each" {
  for_each = var.instance_count # ERROR: number is not valid

  ami           = "ami-12345678"
  instance_type = "t3.micro"
}
