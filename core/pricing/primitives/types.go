// Package primitives - Centralized pricing math
// Mappers declare intent, not do math.
// All pricing logic flows through these primitives.
package primitives

// CloudProvider identifies a cloud provider
type CloudProvider string

const (
	AWS   CloudProvider = "aws"
	Azure CloudProvider = "azure"
	GCP   CloudProvider = "gcp"
)

// TransferDirection for data transfer pricing
type TransferDirection string

const (
	TransferInbound   TransferDirection = "inbound"
	TransferOutbound  TransferDirection = "outbound"
	TransferInterAZ   TransferDirection = "inter_az"
	TransferInterReg  TransferDirection = "inter_region"
	TransferToInternet TransferDirection = "to_internet"
)

// CostUnit represents a billable cost component
type CostUnit struct {
	Name           string
	Measure        string
	Quantity       float64
	RateKey        RateKey
	IsSymbolic     bool
	SymbolicReason string
	Confidence     float64
}

// RateKey identifies a pricing rate
type RateKey struct {
	Provider   string
	Service    string
	Region     string
	Attributes map[string]string
}

// PricingTier represents a tiered pricing level
type PricingTier struct {
	UpTo     float64 // Upper limit (0 = unlimited)
	UnitRate float64 // Rate per unit in this tier
}
