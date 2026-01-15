# Provider Test: Module inheriting provider alias

provider "aws" {
  region = "us-east-1"
}

provider "aws" {
  alias  = "west"
  region = "us-west-2"
}

module "app_east" {
  source = "./app"

  name = "app-east"
}

module "app_west" {
  source = "./app"

  providers = {
    aws = aws.west
  }

  name = "app-west"
}
