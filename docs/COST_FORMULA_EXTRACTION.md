# Cost Formula Extraction Guide

This document explains how to extract pricing knowledge from `combined-resources.txt` (Infracost reference)
WITHOUT copying code.

## The Extraction Process

For each service, extract ONLY:

1. **Pricing Drivers** - Which Terraform attributes affect cost
2. **Usage Dependencies** - Which metrics are needed
3. **Pricing Products** - AWS/Azure/GCP product codes
4. **Cost Formulas** - How quantities map to billing units

## Template: Service Cost Formula

```yaml
service: aws_db_instance
cloud: aws

pricing_drivers:
  - attribute: instance_class
    example: "db.t3.medium"
    affects: hourly_rate
  - attribute: engine
    example: "mysql"
    affects: pricing_product
  - attribute: multi_az
    example: true
    affects: rate_multiplier (2x)
  - attribute: storage_type
    example: "gp2"
    affects: storage_rate
  - attribute: allocated_storage
    example: 100
    affects: storage_quantity
  - attribute: iops
    example: 3000
    affects: iops_quantity (io1 only)

usage_dependencies:
  - metric: monthly_hours
    default: 730
    confidence: 0.95
  - metric: storage_gb
    source: allocated_storage
    confidence: 1.0

cost_units:
  - name: instance
    measure: hours
    formula: monthly_hours
    rate_key:
      service: AmazonRDS
      product: Database Instance
      filters:
        instanceType: {instance_class}
        databaseEngine: {engine}
        deploymentOption: {multi_az ? "Multi-AZ" : "Single-AZ"}

  - name: storage
    measure: GB-months
    formula: allocated_storage
    rate_key:
      service: AmazonRDS
      product: Database Storage
      filters:
        volumeType: {storage_type}

  - name: iops (conditional: storage_type == "io1")
    measure: IOPS-months
    formula: iops
    rate_key:
      service: AmazonRDS
      product: Provisioned IOPS
```

---

## Extracted Formulas

### AWS EC2 (aws_instance)

```yaml
pricing_drivers:
  - instance_type: affects hourly_rate
  - tenancy: "default" | "dedicated" | "host"
  - ami: infers operating_system
  - ebs_optimized: adds surcharge for some types

cost_units:
  - name: compute
    measure: hours
    rate_key: AmazonEC2 / instance_type / os / tenancy

  - name: root_storage
    measure: GB-months
    source: root_block_device.volume_size
    rate_key: AmazonEC2 / EBS:VolumeUsage.{volume_type}
```

### AWS RDS (aws_db_instance)

```yaml
pricing_drivers:
  - instance_class: db.t3.micro, db.r5.large, etc.
  - engine: mysql, postgres, aurora-mysql, oracle-se2, sqlserver-se
  - multi_az: doubles instance cost
  - storage_type: gp2, gp3, io1, magnetic
  - allocated_storage: GB
  - iops: only for io1

cost_units:
  - name: instance
    rate_key: AmazonRDS / {engine} / {instance_class} / {deployment}

  - name: storage
    rate_key: AmazonRDS / {storage_type}

  - name: iops (io1 only)
    rate_key: AmazonRDS / PIOPS

  - name: backup
    rate_key: AmazonRDS / ChargedBackupUsage
```

### AWS S3 (aws_s3_bucket)

```yaml
pricing_drivers:
  - storage_class: STANDARD, INTELLIGENT_TIERING, GLACIER, etc.

usage_dependencies:
  - storage_gb: highly variable
  - put_requests: per 1000
  - get_requests: per 1000
  - data_transfer_out_gb: tiered

cost_units:
  - name: storage
    measure: GB-months
    rate_key: AmazonS3 / TimedStorage-{storage_class}

  - name: put_requests
    measure: requests/1000
    rate_key: AmazonS3 / Requests-Tier1

  - name: get_requests
    measure: requests/1000
    rate_key: AmazonS3 / Requests-Tier2

  - name: data_transfer
    measure: GB
    rate_key: AWSDataTransfer / AWS Outbound
```

### AWS Lambda (aws_lambda_function)

```yaml
pricing_drivers:
  - memory_size: 128-10240 MB
  - architectures: x86_64 | arm64 (20% cheaper)
  - ephemeral_storage.size: >512 MB has cost

usage_dependencies:
  - monthly_requests: symbolic if unknown
  - average_duration_ms: symbolic if unknown

cost_units:
  - name: requests
    measure: requests
    formula: monthly_requests
    rate_key: AWSLambda / Request
    free_tier: 1M requests

  - name: duration
    measure: GB-seconds
    formula: requests * (duration_ms/1000) * (memory_mb/1024)
    rate_key: AWSLambda / {architecture}
    free_tier: 400K GB-seconds

  - name: ephemeral_storage (if >512MB)
    measure: GB-seconds
    rate_key: AWSLambda / Lambda-Provisioned-GB-Second
```

### AWS NAT Gateway (aws_nat_gateway)

```yaml
pricing_drivers:
  - none (fixed pricing per region)

usage_dependencies:
  - monthly_hours: 730
  - data_processed_gb: highly variable

cost_units:
  - name: hourly
    measure: hours
    rate_key: AmazonEC2 / NatGateway-Hours

  - name: data_processed
    measure: GB
    rate_key: AmazonEC2 / NatGateway-Bytes
```

### AWS EKS (aws_eks_cluster)

```yaml
pricing_drivers:
  - none (fixed $0.10/hour per cluster)

cost_units:
  - name: control_plane
    measure: hours
    formula: 730
    rate: $0.10/hour (fixed)
    rate_key: AmazonEKS / AmazonEKS-Hours:perkubernetes
```

### AWS ElastiCache (aws_elasticache_cluster)

```yaml
pricing_drivers:
  - node_type: cache.t3.micro, cache.r5.large, etc.
  - engine: redis | memcached
  - num_cache_nodes: count

cost_units:
  - name: cache_nodes
    measure: node-hours
    formula: num_cache_nodes * monthly_hours
    rate_key: AmazonElastiCache / NodeUsage:{node_type}
```

### AWS DynamoDB (aws_dynamodb_table)

```yaml
pricing_drivers:
  - billing_mode: PROVISIONED | PAY_PER_REQUEST
  - read_capacity: RCU (provisioned only)
  - write_capacity: WCU (provisioned only)

usage_dependencies:
  - storage_gb: grows with data
  - read_request_units: on-demand only
  - write_request_units: on-demand only

cost_units:
  # Provisioned mode
  - name: read_capacity
    measure: RCU-hours
    formula: read_capacity * 730
    rate_key: AmazonDynamoDB / ReadCapacityUnit-Hrs

  - name: write_capacity
    measure: WCU-hours
    formula: write_capacity * 730
    rate_key: AmazonDynamoDB / WriteCapacityUnit-Hrs

  # On-demand mode
  - name: read_requests
    measure: RRU
    rate_key: AmazonDynamoDB / ReadRequestUnit

  - name: write_requests
    measure: WRU
    rate_key: AmazonDynamoDB / WriteRequestUnit

  # Both modes
  - name: storage
    measure: GB-months
    rate_key: AmazonDynamoDB / TimedStorage-ByteHrs
```

---

## What NOT To Copy From Infracost

1. **Struct definitions** - Use our own
2. **Registry patterns** - We have our own
3. **Usage population** - We use explicit UsageContext
4. **Default assumptions** - We emit symbolic costs
5. **Pricing API calls** - We use RateKey abstraction
6. **Error suppression** - We panic on invariant violations
