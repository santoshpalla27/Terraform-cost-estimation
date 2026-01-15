terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

# EC2 Instance
resource "aws_instance" "web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t3.medium"

  root_block_device {
    volume_type = "gp3"
    volume_size = 30
  }

  tags = {
    Name        = "web-server"
    Environment = "production"
  }
}

# RDS Database
resource "aws_db_instance" "main" {
  identifier        = "main-database"
  engine            = "postgres"
  engine_version    = "15.4"
  instance_class    = "db.t3.medium"
  allocated_storage = 100
  storage_type      = "gp2"

  db_name  = "myapp"
  username = "admin"
  password = "changeme123"

  multi_az            = false
  skip_final_snapshot = true

  tags = {
    Name        = "main-database"
    Environment = "production"
  }
}

# NAT Gateway
resource "aws_nat_gateway" "main" {
  allocation_id = "eipalloc-12345678"
  subnet_id     = "subnet-12345678"

  tags = {
    Name = "main-nat"
  }
}

# EBS Volume
resource "aws_ebs_volume" "data" {
  availability_zone = "us-east-1a"
  size              = 500
  type              = "gp3"
  iops              = 3000
  throughput        = 125

  tags = {
    Name = "data-volume"
  }
}

# Lambda Function
resource "aws_lambda_function" "api" {
  function_name = "api-handler"
  role          = "arn:aws:iam::123456789012:role/lambda-role"
  handler       = "index.handler"
  runtime       = "nodejs18.x"
  memory_size   = 256
  timeout       = 30

  filename = "function.zip"

  tags = {
    Name = "api-lambda"
  }
}

# S3 Bucket
resource "aws_s3_bucket" "assets" {
  bucket = "my-app-assets-bucket"

  tags = {
    Name        = "assets"
    Environment = "production"
  }
}
