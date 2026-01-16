# Edge Case: Local values with complex expressions

locals {
  # Nested locals referencing each other
  base_name = "myapp"
  env       = "production"
  full_name = "${local.base_name}-${local.env}"

  # Complex conditional
  instance_size = local.env == "production" ? (
    local.high_availability ? "large" : "medium"
  ) : "small"

  high_availability = local.env == "production"

  # Map transformation
  raw_instances = {
    web    = { count = 3, type = "t3.small" }
    api    = { count = 2, type = "t3.medium" }
    worker = { count = 1, type = "t3.large" }
  }

  # Flatten for iteration
  instance_list = flatten([
    for name, config in local.raw_instances : [
      for i in range(config.count) : {
        name = "${name}-${i}"
        type = config.type
        role = name
      }
    ]
  ])

  # Map by name
  instance_map = {
    for inst in local.instance_list :
    inst.name => inst
  }

  # Sum of counts
  total_instances = sum([for k, v in local.raw_instances : v.count])
}

resource "aws_instance" "from_locals" {
  for_each = local.instance_map

  ami           = "ami-12345678"
  instance_type = each.value.type

  tags = {
    Name = "${local.full_name}-${each.key}"
    Role = each.value.role
  }
}
