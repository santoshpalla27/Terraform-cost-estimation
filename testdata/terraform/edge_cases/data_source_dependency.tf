# Edge Case: Data source dependency

data "aws_ami" "latest" {
  most_recent = true

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }

  owners = ["amazon"]
}

data "aws_vpc" "selected" {
  default = true
}

data "aws_availability_zones" "available" {
  state = "available"
}

resource "aws_instance" "from_data" {
  ami               = data.aws_ami.latest.id
  instance_type     = "t3.micro"
  availability_zone = data.aws_availability_zones.available.names[0]

  vpc_security_group_ids = [data.aws_vpc.selected.default_security_group_id]

  tags = {
    Name = "from-data-sources"
  }
}
