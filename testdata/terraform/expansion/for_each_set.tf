# Expansion Test: for_each with set of strings

variable "availability_zones" {
  type    = set(string)
  default = ["us-east-1a", "us-east-1b", "us-east-1c"]
}

resource "aws_subnet" "main" {
  for_each = var.availability_zones

  vpc_id            = "vpc-12345678"
  cidr_block        = "10.0.${index(tolist(var.availability_zones), each.value)}.0/24"
  availability_zone = each.value

  tags = {
    Name = "subnet-${each.value}"
  }
}

# Resources using the subnets
resource "aws_instance" "per_az" {
  for_each = aws_subnet.main

  ami           = "ami-12345678"
  instance_type = "t3.micro"
  subnet_id     = each.value.id

  tags = {
    Name = "instance-${each.key}"
  }
}
