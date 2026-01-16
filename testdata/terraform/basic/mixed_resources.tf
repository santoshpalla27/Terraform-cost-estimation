# Basic: Mixed resource types for cost estimation

# Compute
resource "aws_instance" "app" {
  count = 3

  ami           = "ami-12345678"
  instance_type = "t3.medium"

  root_block_device {
    volume_size = 30
    volume_type = "gp3"
  }

  tags = {
    Name = "app-${count.index}"
  }
}

# Database
resource "aws_db_instance" "main" {
  identifier     = "main-db"
  engine         = "postgres"
  engine_version = "14.8"
  instance_class = "db.t3.medium"

  allocated_storage     = 100
  max_allocated_storage = 500
  storage_type          = "gp3"

  db_name  = "appdb"
  username = "admin"
  password = "changeme123"

  multi_az            = true
  publicly_accessible = false

  backup_retention_period = 7

  tags = {
    Name = "main-database"
  }
}

# Storage
resource "aws_s3_bucket" "assets" {
  bucket = "my-app-assets-12345"
}

resource "aws_s3_bucket" "logs" {
  bucket = "my-app-logs-12345"
}

# Network
resource "aws_nat_gateway" "main" {
  allocation_id = "eipalloc-12345678"
  subnet_id     = "subnet-12345678"

  tags = {
    Name = "main-nat"
  }
}

resource "aws_lb" "app" {
  name               = "app-lb"
  internal           = false
  load_balancer_type = "application"
  subnets            = ["subnet-12345678", "subnet-87654321"]

  tags = {
    Name = "app-load-balancer"
  }
}

# Cache
resource "aws_elasticache_cluster" "session" {
  cluster_id      = "session-cache"
  engine          = "redis"
  node_type       = "cache.t3.micro"
  num_cache_nodes = 1
  port            = 6379
}

# Lambda
resource "aws_lambda_function" "processor" {
  function_name = "data-processor"
  role          = "arn:aws:iam::123456789:role/lambda-role"
  handler       = "index.handler"
  runtime       = "nodejs18.x"

  memory_size = 256
  timeout     = 30

  filename = "function.zip"
}
