# Basic: High-cost infrastructure pattern

# NAT Gateways (expensive)
resource "aws_nat_gateway" "main" {
  count = 3

  allocation_id = "eipalloc-${count.index}"
  subnet_id     = "subnet-${count.index}"

  tags = {
    Name = "nat-${count.index}"
  }
}

# RDS Multi-AZ (expensive)
resource "aws_db_instance" "production" {
  identifier     = "production-db"
  engine         = "postgres"
  engine_version = "14"
  instance_class = "db.r6g.2xlarge"

  allocated_storage     = 500
  max_allocated_storage = 2000
  storage_type          = "io1"
  iops                  = 10000

  multi_az = true

  backup_retention_period = 30
  storage_encrypted       = true
}

# ElastiCache cluster (medium cost)
resource "aws_elasticache_replication_group" "main" {
  replication_group_id = "main-cache"
  description          = "Main cache cluster"

  node_type          = "cache.r6g.large"
  num_cache_clusters = 3
  engine             = "redis"
  engine_version     = "7.0"

  automatic_failover_enabled = true
  multi_az_enabled           = true
}

# EKS cluster (expensive with nodes)
resource "aws_eks_cluster" "main" {
  name     = "main-cluster"
  role_arn = "arn:aws:iam::123456789:role/eks-cluster-role"

  vpc_config {
    subnet_ids = ["subnet-1", "subnet-2", "subnet-3"]
  }
}

resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "main-nodes"
  node_role_arn   = "arn:aws:iam::123456789:role/eks-node-role"
  subnet_ids      = ["subnet-1", "subnet-2", "subnet-3"]

  scaling_config {
    desired_size = 5
    max_size     = 10
    min_size     = 3
  }

  instance_types = ["m6i.2xlarge"]
}

# Kinesis (usage-based)
resource "aws_kinesis_stream" "events" {
  name             = "event-stream"
  shard_count      = 10
  retention_period = 168

  stream_mode_details {
    stream_mode = "PROVISIONED"
  }
}
