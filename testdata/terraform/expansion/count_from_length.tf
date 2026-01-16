# Expansion: count depending on length of list

variable "subnets" {
  type = list(object({
    cidr = string
    az   = string
  }))
  default = [
    { cidr = "10.0.1.0/24", az = "us-east-1a" },
    { cidr = "10.0.2.0/24", az = "us-east-1b" },
    { cidr = "10.0.3.0/24", az = "us-east-1c" }
  ]
}

resource "aws_subnet" "from_list" {
  count = length(var.subnets)

  vpc_id            = "vpc-12345678"
  cidr_block        = var.subnets[count.index].cidr
  availability_zone = var.subnets[count.index].az

  tags = {
    Name  = "subnet-${count.index}"
    Index = count.index
  }
}

# Instance per subnet
resource "aws_instance" "per_subnet" {
  count = length(aws_subnet.from_list)

  ami           = "ami-12345678"
  instance_type = "t3.micro"
  subnet_id     = aws_subnet.from_list[count.index].id

  tags = {
    Name = "instance-${count.index}"
  }
}
