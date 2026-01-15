# Provider Test: Multiple regions with aliases

provider "aws" {
  region = "us-east-1"
}

provider "aws" {
  alias  = "west"
  region = "us-west-2"
}

provider "aws" {
  alias  = "eu"
  region = "eu-west-1"
}

resource "aws_instance" "east" {
  ami           = "ami-east12345"
  instance_type = "t3.micro"

  tags = {
    Region = "us-east-1"
  }
}

resource "aws_instance" "west" {
  provider      = aws.west
  ami           = "ami-west12345"
  instance_type = "t3.micro"

  tags = {
    Region = "us-west-2"
  }
}

resource "aws_s3_bucket" "eu_bucket" {
  provider = aws.eu
  bucket   = "my-eu-bucket"
}
