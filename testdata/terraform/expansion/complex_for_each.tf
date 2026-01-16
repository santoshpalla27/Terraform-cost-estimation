# Expansion: Complex for_each with nested objects

variable "services" {
  type = map(object({
    cpu         = number
    memory      = number
    ports       = list(number)
    replicas    = number
    environment = map(string)
  }))
  default = {
    api = {
      cpu      = 256
      memory   = 512
      ports    = [8080]
      replicas = 3
      environment = {
        NODE_ENV  = "production"
        LOG_LEVEL = "info"
      }
    }
    worker = {
      cpu      = 512
      memory   = 1024
      ports    = []
      replicas = 2
      environment = {
        QUEUE_URL = "https://sqs.example.com"
      }
    }
    scheduler = {
      cpu         = 128
      memory      = 256
      ports       = []
      replicas    = 1
      environment = {}
    }
  }
}

resource "aws_ecs_task_definition" "services" {
  for_each = var.services

  family                   = each.key
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = each.value.cpu
  memory                   = each.value.memory

  container_definitions = jsonencode([{
    name      = each.key
    image     = "my-app/${each.key}:latest"
    cpu       = each.value.cpu
    memory    = each.value.memory
    essential = true

    portMappings = [
      for port in each.value.ports : {
        containerPort = port
        hostPort      = port
        protocol      = "tcp"
      }
    ]

    environment = [
      for k, v in each.value.environment : {
        name  = k
        value = v
      }
    ]
  }])
}

resource "aws_ecs_service" "services" {
  for_each = var.services

  name            = each.key
  cluster         = "main-cluster"
  task_definition = aws_ecs_task_definition.services[each.key].arn
  desired_count   = each.value.replicas
  launch_type     = "FARGATE"

  network_configuration {
    subnets = ["subnet-12345678"]
  }
}
