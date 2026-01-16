# Expansion: for_each with toset() conversion

variable "bucket_names" {
  type    = list(string)
  default = ["logs", "data", "backups", "archives"]
}

resource "aws_s3_bucket" "buckets" {
  for_each = toset(var.bucket_names)

  bucket = "${each.value}-bucket-12345"

  tags = {
    Name    = each.value
    Purpose = each.key
  }
}

# Dependent resource
resource "aws_s3_bucket_versioning" "buckets" {
  for_each = aws_s3_bucket.buckets

  bucket = each.value.id

  versioning_configuration {
    status = "Enabled"
  }
}
