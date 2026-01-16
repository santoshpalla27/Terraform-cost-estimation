# Edge Case: Terraform workspaces

terraform {
  backend "s3" {
    bucket         = "terraform-state"
    key            = "app/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "terraform-locks"
  }
}

locals {
  workspace_config = {
    default = {
      instance_type  = "t3.micro"
      instance_count = 1
    }
    staging = {
      instance_type  = "t3.small"
      instance_count = 2
    }
    production = {
      instance_type  = "t3.medium"
      instance_count = 3
    }
  }

  current_config = local.workspace_config[terraform.workspace]
}

resource "aws_instance" "app" {
  count = local.current_config.instance_count

  ami           = "ami-12345678"
  instance_type = local.current_config.instance_type

  tags = {
    Name        = "app-${terraform.workspace}-${count.index}"
    Environment = terraform.workspace
  }
}
