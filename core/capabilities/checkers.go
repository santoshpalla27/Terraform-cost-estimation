// Package capabilities - Capability checking utilities
// Provides helper functions to safely check and extract capabilities from assets.
package capabilities

// CheckCompute safely extracts compute sizing from an asset
// Returns nil if asset doesn't implement ComputeSized
func CheckCompute(asset interface{}) (ComputeSized, bool) {
	cs, ok := asset.(ComputeSized)
	return cs, ok
}

// CheckStorage safely extracts storage sizing from an asset
func CheckStorage(asset interface{}) (StorageSized, bool) {
	ss, ok := asset.(StorageSized)
	return ss, ok
}

// CheckIOPS safely extracts provisioned IOPS from an asset
func CheckIOPS(asset interface{}) (IOPSProvisioned, bool) {
	ip, ok := asset.(IOPSProvisioned)
	return ip, ok
}

// CheckNetwork safely extracts network throughput from an asset
func CheckNetwork(asset interface{}) (NetworkThroughput, bool) {
	nt, ok := asset.(NetworkThroughput)
	return nt, ok
}

// CheckMemory safely extracts memory sizing from an asset
func CheckMemory(asset interface{}) (MemorySized, bool) {
	ms, ok := asset.(MemorySized)
	return ms, ok
}

// CheckRegion safely extracts region info from an asset
func CheckRegion(asset interface{}) (RegionAware, bool) {
	ra, ok := asset.(RegionAware)
	return ra, ok
}

// CheckMultiAZ safely extracts multi-AZ config from an asset
func CheckMultiAZ(asset interface{}) (MultiAZ, bool) {
	ma, ok := asset.(MultiAZ)
	return ma, ok
}

// CheckScalable safely extracts scaling config from an asset
func CheckScalable(asset interface{}) (Scalable, bool) {
	s, ok := asset.(Scalable)
	return s, ok
}

// CheckDatabase safely extracts database engine info from an asset
func CheckDatabase(asset interface{}) (DatabaseEngine, bool) {
	de, ok := asset.(DatabaseEngine)
	return de, ok
}

// CheckServerless safely extracts serverless capacity from an asset
func CheckServerless(asset interface{}) (ServerlessCapacity, bool) {
	sc, ok := asset.(ServerlessCapacity)
	return sc, ok
}

// CheckContainer safely extracts container config from an asset
func CheckContainer(asset interface{}) (Containerized, bool) {
	c, ok := asset.(Containerized)
	return c, ok
}

// CheckCache safely extracts cache config from an asset  
func CheckCache(asset interface{}) (Cacheable, bool) {
	c, ok := asset.(Cacheable)
	return c, ok
}

// CheckStream safely extracts streaming config from an asset
func CheckStream(asset interface{}) (Streamable, bool) {
	s, ok := asset.(Streamable)
	return s, ok
}

// CapabilitySet represents the capabilities an asset has
type CapabilitySet struct {
	HasCompute    bool
	HasStorage    bool
	HasIOPS       bool
	HasNetwork    bool
	HasMemory     bool
	HasRegion     bool
	HasMultiAZ    bool
	HasScaling    bool
	HasDatabase   bool
	HasServerless bool
	HasContainer  bool
	HasCache      bool
	HasStream     bool
}

// Analyze determines which capabilities an asset implements
func Analyze(asset interface{}) CapabilitySet {
	return CapabilitySet{
		HasCompute:    implementsCompute(asset),
		HasStorage:    implementsStorage(asset),
		HasIOPS:       implementsIOPS(asset),
		HasNetwork:    implementsNetwork(asset),
		HasMemory:     implementsMemory(asset),
		HasRegion:     implementsRegion(asset),
		HasMultiAZ:    implementsMultiAZ(asset),
		HasScaling:    implementsScaling(asset),
		HasDatabase:   implementsDatabase(asset),
		HasServerless: implementsServerless(asset),
		HasContainer:  implementsContainer(asset),
		HasCache:      implementsCache(asset),
		HasStream:     implementsStream(asset),
	}
}

func implementsCompute(asset interface{}) bool {
	_, ok := asset.(ComputeSized)
	return ok
}

func implementsStorage(asset interface{}) bool {
	_, ok := asset.(StorageSized)
	return ok
}

func implementsIOPS(asset interface{}) bool {
	_, ok := asset.(IOPSProvisioned)
	return ok
}

func implementsNetwork(asset interface{}) bool {
	_, ok := asset.(NetworkThroughput)
	return ok
}

func implementsMemory(asset interface{}) bool {
	_, ok := asset.(MemorySized)
	return ok
}

func implementsRegion(asset interface{}) bool {
	_, ok := asset.(RegionAware)
	return ok
}

func implementsMultiAZ(asset interface{}) bool {
	_, ok := asset.(MultiAZ)
	return ok
}

func implementsScaling(asset interface{}) bool {
	_, ok := asset.(Scalable)
	return ok
}

func implementsDatabase(asset interface{}) bool {
	_, ok := asset.(DatabaseEngine)
	return ok
}

func implementsServerless(asset interface{}) bool {
	_, ok := asset.(ServerlessCapacity)
	return ok
}

func implementsContainer(asset interface{}) bool {
	_, ok := asset.(Containerized)
	return ok
}

func implementsCache(asset interface{}) bool {
	_, ok := asset.(Cacheable)
	return ok
}

func implementsStream(asset interface{}) bool {
	_, ok := asset.(Streamable)
	return ok
}
