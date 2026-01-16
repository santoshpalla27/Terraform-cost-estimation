# Providers: Multiple providers same type

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Default provider
provider "aws" {
  region = "us-east-1"

  default_tags {
    tags = {
      Environment = "production"
      ManagedBy   = "terraform"
    }
  }
}

# Alternative region
provider "aws" {
  alias  = "west"
  region = "us-west-2"

  default_tags {
    tags = {
      Environment = "production"
      ManagedBy   = "terraform"
    }
  }
}

# Cross-account
provider "aws" {
  alias  = "shared"
  region = "us-east-1"

  assume_role {
    role_arn = "arn:aws:iam::987654321:role/terraform-cross-account"
  }
}

# Resources using different providers
resource "aws_s3_bucket" "primary" {
  bucket = "primary-bucket-12345"
}

resource "aws_s3_bucket" "west" {
  provider = aws.west
  bucket   = "west-bucket-12345"
}

resource "aws_s3_bucket" "shared" {
  provider = aws.shared
  bucket   = "shared-bucket-12345"
}

# Replication between regions
resource "aws_s3_bucket_replication_configuration" "primary_to_west" {
  bucket = aws_s3_bucket.primary.id
  role   = "arn:aws:iam::123456789:role/replication-role"

  rule {
    id     = "replicate-to-west"
    status = "Enabled"

    destination {
      bucket        = aws_s3_bucket.west.arn
      storage_class = "STANDARD"
    }
  }
}
