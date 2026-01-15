# Bad Config: Missing required attribute

resource "aws_instance" "missing_ami" {
  # Missing required 'ami' attribute
  instance_type = "t3.micro"
}
