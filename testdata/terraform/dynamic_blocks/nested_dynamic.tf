# Dynamic Block Test: Nested dynamic blocks

variable "load_balancer_config" {
  type = object({
    listeners = list(object({
      port     = number
      protocol = string
      rules = list(object({
        path    = string
        backend = string
      }))
    }))
  })
  default = {
    listeners = [
      {
        port     = 80
        protocol = "HTTP"
        rules = [
          { path = "/api", backend = "api-backend" },
          { path = "/web", backend = "web-backend" }
        ]
      },
      {
        port     = 443
        protocol = "HTTPS"
        rules = [
          { path = "/api", backend = "api-backend" },
          { path = "/admin", backend = "admin-backend" }
        ]
      }
    ]
  }
}

resource "aws_lb_listener" "main" {
  load_balancer_arn = "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/main/abc123"

  dynamic "listener" {
    for_each = var.load_balancer_config.listeners
    content {
      port     = listener.value.port
      protocol = listener.value.protocol

      dynamic "action" {
        for_each = listener.value.rules
        content {
          type = "forward"

          forward {
            target_group_arn = action.value.backend
          }

          condition {
            path_pattern {
              values = [action.value.path]
            }
          }
        }
      }
    }
  }
}
