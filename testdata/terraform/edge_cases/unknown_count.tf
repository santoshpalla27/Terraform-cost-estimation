# Edge Case: Unknown count value

variable "count_from_external" {
  type = number
  # No default - must be provided
}

resource "aws_instance" "unknown_count" {
  count         = var.count_from_external
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}
