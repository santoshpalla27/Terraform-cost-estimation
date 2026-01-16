// Package capabilities - Example asset implementations
// Shows how assets implement capability interfaces
package capabilities

// EC2Asset is an example asset implementing compute capabilities
type EC2Asset struct {
	address      string
	instanceType string
	region       string
	provider     string
}

// NewEC2Asset creates an EC2 asset
func NewEC2Asset(address, instanceType, region string) *EC2Asset {
	return &EC2Asset{
		address:      address,
		instanceType: instanceType,
		region:       region,
		provider:     "aws",
	}
}

// InstanceType implements ComputeSized
func (a *EC2Asset) InstanceType() string {
	return a.instanceType
}

// InstanceCount implements ComputeSized
func (a *EC2Asset) InstanceCount() int {
	return 1
}

// Region implements RegionAware
func (a *EC2Asset) Region() string {
	return a.region
}

// Provider implements RegionAware
func (a *EC2Asset) Provider() string {
	return a.provider
}

// EBSAsset is an example asset implementing storage capabilities
type EBSAsset struct {
	address     string
	volumeType  string
	sizeGB      float64
	iops        int
	throughput  float64
	region      string
}

// NewEBSAsset creates an EBS volume asset
func NewEBSAsset(address, volumeType string, sizeGB float64, iops int, throughput float64, region string) *EBSAsset {
	return &EBSAsset{
		address:    address,
		volumeType: volumeType,
		sizeGB:     sizeGB,
		iops:       iops,
		throughput: throughput,
		region:     region,
	}
}

// StorageGB implements StorageSized
func (a *EBSAsset) StorageGB() float64 {
	return a.sizeGB
}

// StorageType implements StorageSized
func (a *EBSAsset) StorageType() string {
	return a.volumeType
}

// ProvisionedIOPS implements IOPSProvisioned
func (a *EBSAsset) ProvisionedIOPS() int {
	return a.iops
}

// ProvisionedThroughputMBps implements IOPSProvisioned
func (a *EBSAsset) ProvisionedThroughputMBps() float64 {
	return a.throughput
}

// Region implements RegionAware
func (a *EBSAsset) Region() string {
	return a.region
}

// Provider implements RegionAware
func (a *EBSAsset) Provider() string {
	return "aws"
}

// RDSAsset is an example asset implementing database capabilities
type RDSAsset struct {
	address       string
	engine        string
	engineVersion string
	instanceClass string
	storageGB     float64
	storageType   string
	iops          int
	multiAZ       bool
	region        string
}

// NewRDSAsset creates an RDS asset
func NewRDSAsset(address, engine, instanceClass string, storageGB float64, multiAZ bool, region string) *RDSAsset {
	return &RDSAsset{
		address:       address,
		engine:        engine,
		instanceClass: instanceClass,
		storageGB:     storageGB,
		multiAZ:       multiAZ,
		region:        region,
	}
}

// Engine implements DatabaseEngine
func (a *RDSAsset) Engine() string {
	return a.engine
}

// EngineVersion implements DatabaseEngine
func (a *RDSAsset) EngineVersion() string {
	return a.engineVersion
}

// InstanceClass implements DatabaseEngine
func (a *RDSAsset) InstanceClass() string {
	return a.instanceClass
}

// StorageGB implements StorageSized
func (a *RDSAsset) StorageGB() float64 {
	return a.storageGB
}

// StorageType implements StorageSized
func (a *RDSAsset) StorageType() string {
	return a.storageType
}

// IsMultiAZ implements MultiAZ
func (a *RDSAsset) IsMultiAZ() bool {
	return a.multiAZ
}

// ReplicaCount implements MultiAZ
func (a *RDSAsset) ReplicaCount() int {
	if a.multiAZ {
		return 2
	}
	return 1
}

// Region implements RegionAware
func (a *RDSAsset) Region() string {
	return a.region
}

// Provider implements RegionAware
func (a *RDSAsset) Provider() string {
	return "aws"
}
