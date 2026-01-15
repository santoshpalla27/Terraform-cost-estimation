# Edge Case: Conditional resource (count 0 or 1)

variable "create_instance" {
  type    = bool
  default = true
}

variable "create_bucket" {
  type    = bool
  default = false
}

resource "aws_instance" "conditional" {
  count = var.create_instance ? 1 : 0

  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

resource "aws_s3_bucket" "conditional" {
  count = var.create_bucket ? 1 : 0

  bucket = "my-conditional-bucket"
}

# Resource that depends on conditional resource
resource "aws_eip" "instance_eip" {
  count    = var.create_instance ? 1 : 0
  instance = aws_instance.conditional[0].id
}
