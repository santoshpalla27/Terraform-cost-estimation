# Module: Passing count to module

variable "create_vpc" {
  type    = bool
  default = true
}

module "vpc" {
  source = "./vpc"
  count  = var.create_vpc ? 1 : 0

  cidr_block = "10.0.0.0/16"
  name       = "main-vpc"
}

# Accessing module with count
resource "aws_subnet" "public" {
  count = var.create_vpc ? 2 : 0

  vpc_id     = module.vpc[0].vpc_id
  cidr_block = "10.0.${count.index}.0/24"

  tags = {
    Name = "public-${count.index}"
  }
}

# Module with count creating multiple modules
variable "environment_count" {
  type    = number
  default = 3
}

module "environment" {
  source = "./environment"
  count  = var.environment_count

  name  = "env-${count.index}"
  index = count.index
}

output "environment_ids" {
  value = module.environment[*].id
}
