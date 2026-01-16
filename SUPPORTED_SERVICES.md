# Supported Services

This document defines the cost coverage status of the Terraform Cost Estimation platform.

> **Philosophy**: We support ~30 services that cover ~95% of real-world cloud spend.
> Unsupported resources are **explicit**, not hidden.

## Coverage Summary

| Status | AWS | Azure | GCP | Total |
|--------|-----|-------|-----|-------|
| ✅ **Numeric** | 13 | 1 | 1 | 15 |
| ⚠️ **Symbolic** (usage-dependent) | 2 | 0 | 0 | 2 |
| ⬜ **Indirect** (free) | 15+ | - | - | 15+ |
| ❌ **Planned** | 5 | 10+ | 10+ | 25+ |

---

## AWS Services

### ✅ Fully Supported (Numeric Cost)

| Resource Type | Est. Spend | Components | Status |
|--------------|------------|------------|--------|
| `aws_instance` | ~25% | compute, root_storage, ebs_optimized | ✅ |
| `aws_ebs_volume` | ~8% | storage, iops, throughput | ✅ |
| `aws_db_instance` | ~12% | instance, storage, iops, backup | ✅ |
| `aws_dynamodb_table` | ~3% | capacity, storage, replicas | ✅ |
| `aws_s3_bucket` | ~6% | storage, requests, data_transfer | ✅ |
| `aws_lambda_function` | ~4% | requests, duration, ephemeral_storage | ✅ |
| `aws_nat_gateway` | ~5% | hourly, data_processed | ✅ |
| `aws_lb` | ~4% | hourly, LCU | ✅ |
| `aws_eks_cluster` | ~3% | control_plane | ✅ |
| `aws_eks_node_group` | - | nodes (EC2 pricing) | ✅ |
| `aws_elasticache_cluster` | ~2.5% | cache_nodes | ✅ |
| `aws_elasticache_replication_group` | - | cache_nodes (clustered) | ✅ |
| `aws_cloudwatch_metric_alarm` | ~0.5% | alarms | ✅ |
| `aws_autoscaling_group` | indirect | projects instance costs | ✅ |

**Total estimated coverage: ~73% of typical AWS spend**

---

### ⚠️ Usage-Based (Symbolic without usage data)

| Resource Type | Status | Notes |
|--------------|--------|-------|
| `aws_cloudwatch_log_group` | ⚠️ | Requires `monthly_ingestion_gb`, `storage_gb` |

---

### 🔸 Planned (Next Priority)

| Resource Type | Est. Spend | Status |
|--------------|------------|--------|
| `aws_rds_cluster` (Aurora) | ~5% | TODO |
| `aws_redshift_cluster` | ~2% | TODO |
| `aws_opensearch_domain` | ~2% | TODO |
| `aws_kinesis_stream` | ~1.5% | TODO |
| `aws_api_gateway_rest_api` | ~1% | TODO |

**Adding these would bring coverage to ~85%**

---

### ⬜ Indirect Cost (Free Resources)

| Resource Type | Notes |
|--------------|-------|
| `aws_vpc` | VPC itself is free |
| `aws_subnet` | Subnets are free |
| `aws_security_group` | SGs are free |
| `aws_route_table` | Route tables are free |
| `aws_internet_gateway` | IGW is free (data transfer costs) |
| `aws_iam_role` | IAM is free |
| `aws_iam_policy` | IAM is free |
| `aws_launch_template` | Config only |
| `aws_ecs_service` | ECS is free (EC2/Fargate costs) |
| `aws_ecs_task_definition` | Config only |

---

## Azure Services

### 🔸 Placeholder (In Development)

| Resource Type | Status |
|--------------|--------|
| `azurerm_linux_virtual_machine` | Stub |
| `azurerm_storage_account` | TODO |
| `azurerm_sql_database` | TODO |

---

## GCP Services

### 🔸 Placeholder (In Development)

| Resource Type | Status |
|--------------|--------|
| `google_compute_instance` | Stub |
| `google_storage_bucket` | TODO |
| `google_sql_database_instance` | TODO |

---

## Directory Structure

```
clouds/
├── types.go              # Core interfaces
├── registry.go           # Plugin registry
│
├── aws/
│   ├── compute/
│   │   ├── ec2.go        # aws_instance
│   │   └── autoscaling.go
│   ├── storage/
│   │   ├── s3.go
│   │   └── ebs.go
│   ├── database/
│   │   ├── rds.go
│   │   ├── dynamodb.go
│   │   └── elasticache.go
│   ├── networking/
│   │   ├── nat_gateway.go
│   │   └── lb.go
│   ├── containers/
│   │   └── eks.go
│   ├── observability/
│   │   └── cloudwatch.go
│   └── serverless/
│       └── lambda.go
│
├── azure/
│   └── compute/
│       └── vm.go         # Stub
│
└── gcp/
    └── compute/
        └── instance.go   # Stub
```

---

## Coverage Report

Every estimation includes a coverage report:

```
╔════════════════════════════════════════════════════════════╗
║                    COST COVERAGE REPORT                    ║
╠════════════════════════════════════════════════════════════╣
║  Numeric cost:      87.3%  (15 resources)                 ║
║  Symbolic cost:      8.2%  (2 resources)                  ║
║  Unsupported:        4.5%  (1 resource)                   ║
╚════════════════════════════════════════════════════════════╝
```

---

## Strict Mode Thresholds

| Mode | Max Unsupported | Max Symbolic | Min Numeric |
|------|----------------|--------------|-------------|
| Permissive | 100% | 100% | 0% |
| Default | 5% | 10% | 80% |
| Production | 0% | 5% | 95% |

---

## Cost Behavior Classification

| Behavior | Description | Engine Action |
|----------|-------------|---------------|
| `direct` | Always billable | Require mapper |
| `usage_based` | Billable with usage | Mapper + usage data |
| `indirect` | Free, enables costs | Emit zero-cost node |
| `free` | Explicitly free | No cost |
| `unsupported` | Not modeled | Symbolic bucket |

---

## Adding New Services

1. **Classify** in `core/coverage/aws_profiles.go`
2. **Implement mapper** in `clouds/aws/<category>/<service>.go`
3. **Add tests** with Terraform examples
4. **Update this document**

Priority: **Cost impact > feature count**
