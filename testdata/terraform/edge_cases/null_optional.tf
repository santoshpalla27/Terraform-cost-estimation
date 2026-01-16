# Edge Case: Null values and optionals

variable "optional_config" {
  type = object({
    enabled       = optional(bool, false)
    instance_type = optional(string)
    volume_size   = optional(number, 20)
  })
  default = {}
}

variable "nullable_string" {
  type    = string
  default = null
}

resource "aws_instance" "optional_test" {
  count = var.optional_config.enabled ? 1 : 0

  ami           = "ami-12345678"
  instance_type = coalesce(var.optional_config.instance_type, "t3.micro")

  root_block_device {
    volume_size = var.optional_config.volume_size
  }

  # Using try() for safe access
  user_data = try(var.nullable_string, "default-user-data")

  tags = {
    Name = "optional-test"
  }
}

# Null resource for testing
resource "null_resource" "example" {
  count = var.nullable_string != null ? 1 : 0

  triggers = {
    value = var.nullable_string
  }
}
