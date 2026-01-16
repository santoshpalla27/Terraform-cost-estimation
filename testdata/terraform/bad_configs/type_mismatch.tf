# Bad Config: Type mismatch

variable "count_value" {
  type    = string  # Should be number
  default = "three" # Invalid for count
}

resource "aws_instance" "type_error" {
  count = var.count_value # ERROR: string to number

  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

variable "for_each_value" {
  type    = number
  default = 5
}

resource "aws_instance" "for_each_error" {
  for_each = var.for_each_value # ERROR: number not valid

  ami           = "ami-12345678"
  instance_type = "t3.micro"
}
