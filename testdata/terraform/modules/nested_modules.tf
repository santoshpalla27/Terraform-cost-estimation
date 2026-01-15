# Module Test: Nested module with for_each

variable "environments" {
  type = map(object({
    instance_count = number
    instance_type  = string
  }))
  default = {
    dev = {
      instance_count = 1
      instance_type  = "t3.micro"
    }
    staging = {
      instance_count = 2
      instance_type  = "t3.small"
    }
    prod = {
      instance_count = 3
      instance_type  = "t3.medium"
    }
  }
}

module "environment" {
  source   = "./environment"
  for_each = var.environments

  name           = each.key
  instance_count = each.value.instance_count
  instance_type  = each.value.instance_type
}
