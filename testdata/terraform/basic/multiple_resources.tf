# Basic Test: Multiple resource types

resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.small"

  root_block_device {
    volume_size = 20
    volume_type = "gp3"
  }

  tags = {
    Name = "web-server"
  }
}

resource "aws_ebs_volume" "data" {
  availability_zone = "us-east-1a"
  size              = 100
  type              = "gp3"

  tags = {
    Name = "data-volume"
  }
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.web.id
}

resource "aws_s3_bucket" "logs" {
  bucket = "my-app-logs-bucket"
}

resource "aws_rds_instance" "db" {
  identifier     = "mydb"
  engine         = "mysql"
  engine_version = "8.0"
  instance_class = "db.t3.micro"

  allocated_storage = 20
  storage_type      = "gp2"

  tags = {
    Name = "app-database"
  }
}
