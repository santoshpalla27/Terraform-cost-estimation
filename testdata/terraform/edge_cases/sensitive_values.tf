# Edge Case: Sensitive values

variable "db_password" {
  type      = string
  sensitive = true
}

variable "api_key" {
  type      = string
  sensitive = true
  default   = "default-key"
}

resource "aws_db_instance" "sensitive" {
  identifier     = "sensitive-db"
  engine         = "mysql"
  instance_class = "db.t3.micro"

  username = "admin"
  password = var.db_password # Sensitive

  tags = {
    Name = "sensitive-db"
  }
}

resource "aws_secretsmanager_secret" "api" {
  name = "api-credentials"
}

resource "aws_secretsmanager_secret_version" "api" {
  secret_id = aws_secretsmanager_secret.api.id
  secret_string = jsonencode({
    api_key = var.api_key # Sensitive
  })
}
