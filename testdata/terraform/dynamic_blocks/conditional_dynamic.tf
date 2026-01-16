# Dynamic Block: Conditional dynamic block

variable "enable_logging" {
  type    = bool
  default = true
}

variable "access_logs_bucket" {
  type    = string
  default = "my-access-logs"
}

variable "enable_stickiness" {
  type    = bool
  default = false
}

resource "aws_lb" "conditional_dynamic" {
  name               = "conditional-lb"
  internal           = false
  load_balancer_type = "application"
  subnets            = ["subnet-12345678", "subnet-87654321"]

  # Conditional dynamic block for access logs
  dynamic "access_logs" {
    for_each = var.enable_logging ? [1] : []
    content {
      bucket  = var.access_logs_bucket
      prefix  = "lb-logs"
      enabled = true
    }
  }
}

resource "aws_lb_target_group" "conditional" {
  name     = "conditional-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = "vpc-12345678"

  # Conditional stickiness
  dynamic "stickiness" {
    for_each = var.enable_stickiness ? [1] : []
    content {
      type            = "lb_cookie"
      cookie_duration = 86400
      enabled         = true
    }
  }
}
