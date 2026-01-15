# Expansion Test: for_each with map

variable "instances" {
  type = map(object({
    instance_type = string
    ami           = string
  }))
  default = {
    web = {
      instance_type = "t3.small"
      ami           = "ami-web12345"
    }
    api = {
      instance_type = "t3.medium"
      ami           = "ami-api12345"
    }
    worker = {
      instance_type = "t3.large"
      ami           = "ami-wrk12345"
    }
  }
}

resource "aws_instance" "multi" {
  for_each      = var.instances
  ami           = each.value.ami
  instance_type = each.value.instance_type

  tags = {
    Name = "instance-${each.key}"
    Role = each.key
  }
}
