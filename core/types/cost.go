// Package types - Cost graph types
package types

import "github.com/shopspring/decimal"

// Currency represents a currency code
type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
)

// String returns the string representation
func (c Currency) String() string {
	return string(c)
}

// RateKey uniquely identifies a pricing rate
type RateKey struct {
	// Provider is the cloud provider
	Provider Provider `json:"provider"`

	// Service is the cloud service (e.g., "EC2", "S3")
	Service string `json:"service"`

	// ProductFamily is the product family (e.g., "Compute Instance", "Storage")
	ProductFamily string `json:"product_family"`

	// Region is the cloud region
	Region string `json:"region"`

	// Attributes contains SKU-specific attributes
	Attributes map[string]string `json:"attributes,omitempty"`
}

// String returns a string representation for caching/lookup
func (k RateKey) String() string {
	return string(k.Provider) + "/" + k.Service + "/" + k.ProductFamily + "/" + k.Region
}

// CostUnit represents a single billable line item
type CostUnit struct {
	// ID uniquely identifies this cost unit
	ID string `json:"id"`

	// Label is a human-readable label
	Label string `json:"label"`

	// Description provides additional context
	Description string `json:"description,omitempty"`

	// Measure is the billing unit (e.g., "GB-month", "hours", "requests")
	Measure string `json:"measure"`

	// Quantity is the usage quantity
	Quantity decimal.Decimal `json:"quantity"`

	// RateKey identifies the pricing rate
	RateKey RateKey `json:"rate_key"`

	// Rate is the unit price
	Rate decimal.Decimal `json:"rate"`

	// Amount is the calculated cost (Quantity * Rate)
	Amount decimal.Decimal `json:"amount"`

	// Currency is the cost currency
	Currency Currency `json:"currency"`

	// Lineage tracks why this cost exists
	Lineage CostLineage `json:"lineage"`

	// IsSubcost indicates if this is a sub-component of a larger cost
	IsSubcost bool `json:"is_subcost,omitempty"`
}

// CostLineage tracks the origin and calculation of a cost
type CostLineage struct {
	// AssetID links to the source asset
	AssetID string `json:"asset_id"`

	// AssetAddress is the Terraform resource address
	AssetAddress ResourceAddress `json:"asset_address"`

	// Formula describes how the cost was calculated
	Formula string `json:"formula"`

	// UsageVector is the usage data used in calculation
	UsageVector *UsageVector `json:"usage_vector,omitempty"`

	// Assumptions lists assumptions made during calculation
	Assumptions []string `json:"assumptions,omitempty"`
}

// CostAggregate groups cost units by a dimension
type CostAggregate struct {
	// ID uniquely identifies this aggregate
	ID string `json:"id"`

	// Label is a human-readable label
	Label string `json:"label"`

	// Description provides additional context
	Description string `json:"description,omitempty"`

	// Units contains the cost units in this aggregate
	Units []*CostUnit `json:"units,omitempty"`

	// Children contains child aggregates
	Children []*CostAggregate `json:"children,omitempty"`

	// MonthlyCost is the calculated monthly total
	MonthlyCost decimal.Decimal `json:"monthly_cost"`

	// HourlyCost is the calculated hourly cost
	HourlyCost decimal.Decimal `json:"hourly_cost"`

	// Currency is the cost currency
	Currency Currency `json:"currency"`
}

// Add adds a cost unit to the aggregate
func (a *CostAggregate) Add(unit *CostUnit) {
	a.Units = append(a.Units, unit)
	a.MonthlyCost = a.MonthlyCost.Add(unit.Amount)
}

// Total returns the total monthly cost
func (a *CostAggregate) Total() decimal.Decimal {
	total := a.MonthlyCost
	for _, child := range a.Children {
		total = total.Add(child.Total())
	}
	return total
}

// CostGraph represents the complete cost model as a graph
type CostGraph struct {
	// Root is the top-level aggregate
	Root *CostAggregate `json:"root"`

	// ByAsset groups costs by asset ID
	ByAsset map[string]*CostAggregate `json:"by_asset,omitempty"`

	// ByProvider groups costs by cloud provider
	ByProvider map[Provider]*CostAggregate `json:"by_provider,omitempty"`

	// ByService groups costs by service
	ByService map[string]*CostAggregate `json:"by_service,omitempty"`

	// ByCategory groups costs by asset category
	ByCategory map[AssetCategory]*CostAggregate `json:"by_category,omitempty"`

	// TotalMonthlyCost is the overall monthly total
	TotalMonthlyCost decimal.Decimal `json:"total_monthly_cost"`

	// TotalHourlyCost is the overall hourly total
	TotalHourlyCost decimal.Decimal `json:"total_hourly_cost"`

	// Currency is the primary currency
	Currency Currency `json:"currency"`

	// Metadata contains graph-level information
	Metadata CostGraphMetadata `json:"metadata"`
}

// CostGraphMetadata contains metadata about the cost graph
type CostGraphMetadata struct {
	// PricingSnapshotID is the pricing snapshot used
	PricingSnapshotID string `json:"pricing_snapshot_id"`

	// CreatedAt is when the graph was created
	CreatedAt string `json:"created_at"`

	// AssetCount is the number of priced assets
	AssetCount int `json:"asset_count"`

	// CostUnitCount is the total number of cost units
	CostUnitCount int `json:"cost_unit_count"`

	// MissingPrices lists rate keys that couldn't be resolved
	MissingPrices []RateKey `json:"missing_prices,omitempty"`
}

// NewCostGraph creates a new empty cost graph
func NewCostGraph(currency Currency) *CostGraph {
	return &CostGraph{
		Root: &CostAggregate{
			ID:       "root",
			Label:    "Total",
			Currency: currency,
		},
		ByAsset:    make(map[string]*CostAggregate),
		ByProvider: make(map[Provider]*CostAggregate),
		ByService:  make(map[string]*CostAggregate),
		ByCategory: make(map[AssetCategory]*CostAggregate),
		Currency:   currency,
	}
}

// AddCostUnit adds a cost unit to the graph with proper indexing
func (g *CostGraph) AddCostUnit(unit *CostUnit, asset *Asset) {
	// Add to root
	g.Root.Add(unit)

	// Add to by-asset index
	if _, ok := g.ByAsset[asset.ID]; !ok {
		g.ByAsset[asset.ID] = &CostAggregate{
			ID:       asset.ID,
			Label:    string(asset.Address),
			Currency: g.Currency,
		}
	}
	g.ByAsset[asset.ID].Add(unit)

	// Add to by-provider index
	if _, ok := g.ByProvider[asset.Provider]; !ok {
		g.ByProvider[asset.Provider] = &CostAggregate{
			ID:       string(asset.Provider),
			Label:    string(asset.Provider),
			Currency: g.Currency,
		}
	}
	g.ByProvider[asset.Provider].Add(unit)

	// Add to by-service index
	service := unit.RateKey.Service
	if _, ok := g.ByService[service]; !ok {
		g.ByService[service] = &CostAggregate{
			ID:       service,
			Label:    service,
			Currency: g.Currency,
		}
	}
	g.ByService[service].Add(unit)

	// Add to by-category index
	if _, ok := g.ByCategory[asset.Category]; !ok {
		g.ByCategory[asset.Category] = &CostAggregate{
			ID:       string(asset.Category),
			Label:    string(asset.Category),
			Currency: g.Currency,
		}
	}
	g.ByCategory[asset.Category].Add(unit)

	// Update totals
	g.TotalMonthlyCost = g.TotalMonthlyCost.Add(unit.Amount)
	g.Metadata.CostUnitCount++
}

// Summarize recalculates all totals in the graph
func (g *CostGraph) Summarize() {
	g.TotalMonthlyCost = g.Root.Total()
	// Convert to hourly (730 hours/month)
	g.TotalHourlyCost = g.TotalMonthlyCost.Div(decimal.NewFromInt(730))
}
