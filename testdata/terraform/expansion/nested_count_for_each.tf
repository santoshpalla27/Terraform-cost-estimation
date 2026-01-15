# Expansion Test: Nested count inside for_each

variable "environments" {
  type = map(object({
    replica_count = number
  }))
  default = {
    dev     = { replica_count = 1 }
    staging = { replica_count = 2 }
    prod    = { replica_count = 3 }
  }
}

# Outer for_each
resource "aws_db_instance" "primary" {
  for_each = var.environments

  identifier     = "${each.key}-db-primary"
  engine         = "mysql"
  instance_class = each.key == "prod" ? "db.r5.large" : "db.t3.micro"

  tags = {
    Environment = each.key
    Role        = "primary"
  }
}

# This pattern (count inside module with for_each) is common
module "db_replicas" {
  source   = "./replica"
  for_each = var.environments

  count       = each.value.replica_count
  primary_id  = aws_db_instance.primary[each.key].id
  environment = each.key
}
