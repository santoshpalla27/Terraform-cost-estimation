// Package aws - AWS cloud provider registration
package aws

// SupportedResourceTypes returns all supported AWS resource types
func SupportedResourceTypes() []string {
	return []string{
		// Compute
		"aws_instance",
		"aws_autoscaling_group",
		"aws_spot_instance_request",

		// Storage
		"aws_s3_bucket",
		"aws_ebs_volume",

		// Database
		"aws_db_instance",
		"aws_rds_cluster",
		"aws_dynamodb_table",

		// Serverless
		"aws_lambda_function",
	}
}
