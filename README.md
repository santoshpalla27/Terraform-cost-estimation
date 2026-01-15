# Terraform Cost Estimation System

A production-grade, cloud-agnostic infrastructure cost estimation tool for Terraform configurations.

## Features

- **Cloud-Agnostic Core**: Support for AWS, Azure, and GCP (AWS fully implemented)
- **Asset Graph**: Provider-agnostic infrastructure DAG for diff and explainability
- **Cost Graph**: Full lineage tracking for every cost calculation
- **Reproducible Estimates**: Versioned pricing snapshots
- **Policy Engine**: Budget limits, thresholds, and guardrails
- **Multiple Outputs**: CLI tables, JSON, HTML, PR comments

## Quick Start

### Using Docker (Recommended)

```bash
# Build the image
docker build -t terraform-cost .

# Run estimation on a Terraform project
docker run -v /path/to/your/terraform:/projects terraform-cost estimate /projects

# Using docker-compose
docker-compose run terraform-cost estimate /projects
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/your-org/terraform-cost.git
cd terraform-cost

# Install dependencies
go mod download

# Build
go build -o terraform-cost ./cmd/cli

# Run
./terraform-cost estimate ./your-terraform-project
```

## Usage

```bash
# Basic estimation
terraform-cost estimate ./my-terraform-project

# JSON output
terraform-cost estimate --format json ./infrastructure

# With custom usage file
terraform-cost estimate --usage usage.yml ./infrastructure

# Show version
terraform-cost version
```

## Project Structure

```
terraform-cost/
├── cmd/                    # CLI and server entry points
│   ├── cli/
│   └── server/
├── core/                   # Cloud-agnostic core engine
│   ├── types/              # Domain types
│   ├── scanner/            # Infrastructure scanning
│   ├── asset/              # Asset graph
│   ├── usage/              # Usage estimation
│   ├── cost/               # Cost calculation
│   ├── pricing/            # Pricing resolution
│   ├── policy/             # Policy evaluation
│   └── output/             # Output formatting
├── clouds/                 # Cloud provider plugins
│   ├── aws/
│   ├── azure/
│   └── gcp/
├── adapters/               # External system adapters
│   └── terraform/
├── internal/               # Internal utilities
└── examples/               # Example Terraform configs
```

## Supported AWS Resources

| Category   | Resources |
|------------|-----------|
| Compute    | EC2, Auto Scaling, Lambda, ECS, EKS |
| Storage    | S3, EBS, EFS |
| Database   | RDS, DynamoDB, ElastiCache |
| Network    | NAT Gateway, VPC Endpoints, ALB/NLB/ELB |
| Security   | KMS, Secrets Manager |
| Monitoring | CloudWatch Log Groups |

## Architecture

```
Input → Scanner → Asset Graph → Usage Estimation → Cost Graph → Pricing → Policy → Output
```

### Key Design Principles

1. **Hard Separation of Concerns**: Scanners don't know about pricing
2. **Cloud Providers are Plugins**: Core engine is cloud-agnostic
3. **Deterministic & Reproducible**: Every estimate uses versioned pricing
4. **Everything is a Graph**: Enables diff, lineage, and explainability

## Configuration

Create `.terraform-cost.json` in your home directory or project root:

```json
{
  "pricing": {
    "default_currency": "USD",
    "cache_enabled": true
  },
  "output": {
    "default_format": "cli",
    "show_details": true
  },
  "aws": {
    "default_region": "us-east-1"
  }
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) for details.
