# Edge Case: Resource moved block

# Simulate state moves
moved {
  from = aws_instance.old_name
  to   = aws_instance.new_name
}

resource "aws_instance" "new_name" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"

  tags = {
    Name = "renamed-instance"
  }
}

# Import block
import {
  to = aws_s3_bucket.imported
  id = "my-existing-bucket"
}

resource "aws_s3_bucket" "imported" {
  bucket = "my-existing-bucket"
}
