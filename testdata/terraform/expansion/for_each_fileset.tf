# Expansion: for_each with fileset

resource "aws_s3_object" "configs" {
  for_each = fileset("${path.module}/configs", "*.json")

  bucket = "my-config-bucket"
  key    = "configs/${each.value}"
  source = "${path.module}/configs/${each.value}"

  etag = filemd5("${path.module}/configs/${each.value}")
}

resource "aws_lambda_function" "handlers" {
  for_each = fileset("${path.module}/lambdas", "*/handler.py")

  function_name = replace(dirname(each.value), "/", "-")
  role          = "arn:aws:iam::123456789:role/lambda-role"
  handler       = "handler.main"
  runtime       = "python3.9"

  filename = "${path.module}/lambdas/${each.value}"
}
