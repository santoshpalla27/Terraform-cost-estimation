# Module: Passing providers to module

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  alias  = "primary"
  region = "us-east-1"
}

provider "aws" {
  alias  = "dr"
  region = "us-west-2"
}

module "primary_infra" {
  source = "./infra"

  providers = {
    aws = aws.primary
  }

  environment = "primary"
  vpc_cidr    = "10.0.0.0/16"
}

module "dr_infra" {
  source = "./infra"

  providers = {
    aws = aws.dr
  }

  environment = "dr"
  vpc_cidr    = "10.1.0.0/16"
}

# Cross-region peering would reference both
resource "aws_vpc_peering_connection" "primary_to_dr" {
  provider = aws.primary

  vpc_id      = module.primary_infra.vpc_id
  peer_vpc_id = module.dr_infra.vpc_id
  peer_region = "us-west-2"

  tags = {
    Name = "primary-to-dr"
  }
}
