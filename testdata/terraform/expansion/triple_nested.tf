# Expansion: Triple nested for_each/count

variable "regions" {
  type = map(object({
    availability_zones = list(string)
    instance_per_az    = number
  }))
  default = {
    us-east-1 = {
      availability_zones = ["us-east-1a", "us-east-1b"]
      instance_per_az    = 2
    }
    us-west-2 = {
      availability_zones = ["us-west-2a"]
      instance_per_az    = 3
    }
  }
}

# Level 1: for_each on regions
module "regional" {
  source   = "./regional"
  for_each = var.regions

  region             = each.key
  availability_zones = each.value.availability_zones
  instance_per_az    = each.value.instance_per_az
}

# Simulating what the regional module might contain:
# Level 2: for_each on AZs (conceptually)
# Level 3: count on instances per AZ

# This flattening pattern is common
locals {
  all_instances = flatten([
    for region, config in var.regions : [
      for az in config.availability_zones : [
        for i in range(config.instance_per_az) : {
          region = region
          az     = az
          index  = i
          name   = "${region}-${az}-${i}"
        }
      ]
    ]
  ])
}

resource "aws_instance" "triple_nested" {
  for_each = { for inst in local.all_instances : inst.name => inst }

  ami               = "ami-12345678"
  instance_type     = "t3.micro"
  availability_zone = each.value.az

  tags = {
    Name   = each.key
    Region = each.value.region
    Index  = each.value.index
  }
}
