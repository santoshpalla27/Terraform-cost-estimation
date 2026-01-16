// Package capabilities - Asset capability interfaces
// Formalizes what assets can provide, eliminating fragile attribute assumptions.
// No interface → mapper cannot consume it.
package capabilities

// ComputeSized indicates the asset has compute sizing information
type ComputeSized interface {
	// InstanceType returns the instance/VM type (e.g., "t3.micro", "Standard_B1s")
	InstanceType() string

	// InstanceCount returns the number of instances (1 for single, N for autoscaling)
	InstanceCount() int
}

// StorageSized indicates the asset has storage capacity
type StorageSized interface {
	// StorageGB returns the storage capacity in GB
	StorageGB() float64

	// StorageType returns the storage type (e.g., "gp3", "io1", "Standard_LRS")
	StorageType() string
}

// IOPSProvisioned indicates the asset has provisioned IOPS
type IOPSProvisioned interface {
	// ProvisionedIOPS returns the provisioned IOPS count
	ProvisionedIOPS() int

	// ProvisionedThroughputMBps returns provisioned throughput in MB/s (0 if not applicable)
	ProvisionedThroughputMBps() float64
}

// NetworkThroughput indicates the asset has network usage
type NetworkThroughput interface {
	// MonthlyDataTransferGB returns estimated monthly data transfer in GB
	MonthlyDataTransferGB() float64

	// TransferDirection returns the primary transfer direction
	TransferDirection() string
}

// MemorySized indicates the asset has memory sizing
type MemorySized interface {
	// MemoryMB returns the memory in MB
	MemoryMB() int
}

// RegionAware indicates the asset knows its region
type RegionAware interface {
	// Region returns the deployment region
	Region() string

	// Provider returns the cloud provider
	Provider() string
}

// MultiAZ indicates the asset has multi-AZ configuration
type MultiAZ interface {
	// IsMultiAZ returns true if multi-AZ is enabled
	IsMultiAZ() bool

	// ReplicaCount returns the number of replicas
	ReplicaCount() int
}

// Scalable indicates the asset can scale
type Scalable interface {
	// MinCapacity returns minimum capacity
	MinCapacity() int

	// MaxCapacity returns maximum capacity
	MaxCapacity() int

	// DesiredCapacity returns desired/current capacity
	DesiredCapacity() int

	// IsFixedSize returns true if min == max
	IsFixedSize() bool
}

// UsageDependent indicates the asset requires usage data
type UsageDependent interface {
	// RequiredUsageMetrics returns the list of required usage metrics
	RequiredUsageMetrics() []string

	// OptionalUsageMetrics returns the list of optional metrics
	OptionalUsageMetrics() []string
}

// DatabaseEngine indicates the asset is a database
type DatabaseEngine interface {
	// Engine returns the database engine (mysql, postgres, etc.)
	Engine() string

	// EngineVersion returns the engine version
	EngineVersion() string

	// InstanceClass returns the database instance class
	InstanceClass() string
}

// ServerlessCapacity indicates serverless scaling configuration
type ServerlessCapacity interface {
	// MinACU returns minimum capacity units
	MinACU() float64

	// MaxACU returns maximum capacity units
	MaxACU() float64

	// IsServerless returns true if running in serverless mode
	IsServerless() bool
}

// Containerized indicates the asset runs containers
type Containerized interface {
	// VCPU returns the vCPU allocation
	VCPU() float64

	// MemoryGB returns the memory allocation in GB
	MemoryGB() float64

	// ContainerCount returns number of containers/tasks
	ContainerCount() int
}

// Cacheable indicates the asset is a caching layer
type Cacheable interface {
	// CacheEngine returns the cache engine (redis, memcached)
	CacheEngine() string

	// NodeType returns the cache node type
	NodeType() string

	// NodeCount returns the number of cache nodes
	NodeCount() int
}

// Streamable indicates the asset handles streaming data
type Streamable interface {
	// ShardCount returns number of shards
	ShardCount() int

	// RetentionHours returns data retention in hours
	RetentionHours() int
}
