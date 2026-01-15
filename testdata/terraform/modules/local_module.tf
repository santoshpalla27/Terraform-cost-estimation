# Module Test: Local module with count

variable "app_count" {
  type    = number
  default = 2
}

module "app" {
  source = "./app"
  count  = var.app_count

  name          = "app-${count.index}"
  instance_type = "t3.micro"
}

output "app_ids" {
  value = module.app[*].instance_id
}
