# Edge Case: Self-referencing count

resource "aws_instance" "self_ref" {
  count = length(aws_instance.self_ref) > 0 ? 2 : 1 # This is invalid but should be handled

  ami           = "ami-12345678"
  instance_type = "t3.micro"
}
