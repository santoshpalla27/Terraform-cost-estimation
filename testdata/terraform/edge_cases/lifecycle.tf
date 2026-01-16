# Edge Case: Lifecycle with create_before_destroy

resource "aws_instance" "lifecycle_test" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"

  lifecycle {
    create_before_destroy = true
    prevent_destroy       = false
    ignore_changes        = [tags, user_data]
  }

  tags = {
    Name = "lifecycle-test"
  }
}

resource "aws_launch_template" "lifecycle" {
  name_prefix   = "lifecycle-"
  instance_type = "t3.micro"
  image_id      = "ami-12345678"

  lifecycle {
    create_before_destroy = true
  }
}
