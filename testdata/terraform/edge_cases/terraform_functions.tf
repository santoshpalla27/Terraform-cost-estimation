# Edge Case: Terraform functions in expressions

locals {
  environment = "production"

  # String functions
  upper_env = upper(local.environment)
  lower_env = lower(local.environment)

  # Collection functions
  instance_types = ["t3.micro", "t3.small", "t3.medium"]
  first_type     = element(local.instance_types, 0)
  type_count     = length(local.instance_types)

  # Numeric functions
  max_cpu = max(2, 4, 8)
  min_cpu = min(2, 4, 8)

  # Conditional
  instance_type = local.environment == "production" ? "t3.large" : "t3.micro"

  # Map functions
  tags = merge(
    {
      Environment = local.environment
    },
    {
      ManagedBy = "terraform"
    }
  )

  # Encoding
  encoded = base64encode("hello world")
}

resource "aws_instance" "with_functions" {
  ami           = "ami-12345678"
  instance_type = local.instance_type

  tags = merge(local.tags, {
    Name = format("web-%s-%d", local.environment, 1)
  })
}
