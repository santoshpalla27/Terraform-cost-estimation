// Package cost - Normalized cost graph
// CostUnit (atomic) → CostNode (grouped by asset) → CostAggregate (service/provider/project)
// No pricing logic exists above CostUnit.
package cost

import (
	"sort"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
)

// CostUnit is the atomic unit of cost
// All pricing logic lives here and ONLY here
type CostUnit struct {
	// Identity
	ID       string
	Category CostCategory

	// What this costs
	Amount   determinism.Money
	Hourly   determinism.Money
	Monthly  determinism.Money

	// Pricing derivation (immutable)
	Rate     *RateDerivation
	Usage    *UsageDerivation
	Formula  *FormulaDerivation

	// Confidence
	Confidence float64
	Factors    []CostConfidenceFactor

	// Is this an assumption?
	IsAssumed    bool
	AssumptionID string
}

// CostCategory classifies cost units
type CostCategory int

const (
	CategoryCompute   CostCategory = iota // EC2, Lambda, ECS
	CategoryStorage                        // S3, EBS, RDS storage
	CategoryNetwork                        // NAT, data transfer
	CategoryDatabase                       // RDS, DynamoDB, ElastiCache
	CategoryCache                          // ElastiCache, DAX
	CategoryMessaging                      // SQS, SNS, Kinesis
	CategoryOther                          // Uncategorized
)

// String returns the category name
func (c CostCategory) String() string {
	switch c {
	case CategoryCompute:
		return "compute"
	case CategoryStorage:
		return "storage"
	case CategoryNetwork:
		return "network"
	case CategoryDatabase:
		return "database"
	case CategoryCache:
		return "cache"
	case CategoryMessaging:
		return "messaging"
	default:
		return "other"
	}
}

// RateDerivation records how the rate was determined
type RateDerivation struct {
	RateID       string
	SKU          string
	Price        determinism.Money
	Unit         string
	Region       string
	SnapshotID   string
	SnapshotHash string
}

// UsageDerivation records how usage was determined
type UsageDerivation struct {
	Value     float64
	Unit      string
	Source    CostUsageSource
	IsDefault bool
	DefaultID string
}

// CostUsageSource indicates where usage came from
type CostUsageSource int

const (
	CostUsageFromConfig     CostUsageSource = iota // From Terraform config
	CostUsageFromDefault                            // From default value
	CostUsageFromOverride                           // From usage file
	CostUsageFromHistorical                         // From historical data
)

// FormulaDerivation records the calculation formula
type FormulaDerivation struct {
	Expression string   // e.g., "rate * usage * hours"
	Variables  []string // Variables used
	Result     string   // Result expression
}

// CostConfidenceFactor records a reason for confidence change
type CostConfidenceFactor struct {
	Rule   string
	Reason string
	Impact float64
}

// CostNode groups CostUnits by asset
type CostNode struct {
	// Identity
	InstanceID      model.InstanceID
	InstanceAddress model.InstanceAddress
	ResourceType    string
	Provider        string
	Region          string

	// Cost units for this asset
	Units []*CostUnit

	// Aggregated costs
	TotalMonthly determinism.Money
	TotalHourly  determinism.Money

	// Category breakdown
	ByCategory map[CostCategory]determinism.Money

	// Aggregated confidence (minimum)
	Confidence float64

	// Assumptions made
	Assumptions []string
}

// NewCostNode creates a new cost node
func NewCostNode(id model.InstanceID, address model.InstanceAddress, resourceType, provider, region string) *CostNode {
	return &CostNode{
		InstanceID:      id,
		InstanceAddress: address,
		ResourceType:    resourceType,
		Provider:        provider,
		Region:          region,
		Units:           []*CostUnit{},
		ByCategory:      make(map[CostCategory]determinism.Money),
		Confidence:      1.0,
		Assumptions:     []string{},
	}
}

// AddUnit adds a cost unit and updates aggregates
func (n *CostNode) AddUnit(unit *CostUnit) {
	n.Units = append(n.Units, unit)
	n.TotalMonthly = n.TotalMonthly.Add(unit.Monthly)
	n.TotalHourly = n.TotalHourly.Add(unit.Hourly)

	// Update category breakdown
	if existing, ok := n.ByCategory[unit.Category]; ok {
		n.ByCategory[unit.Category] = existing.Add(unit.Monthly)
	} else {
		n.ByCategory[unit.Category] = unit.Monthly
	}

	// Update confidence (take minimum)
	if unit.Confidence < n.Confidence {
		n.Confidence = unit.Confidence
	}

	// Track assumptions
	if unit.IsAssumed {
		n.Assumptions = append(n.Assumptions, unit.AssumptionID)
	}
}

// CostAggregate groups CostNodes by service/provider/project
type CostAggregate struct {
	// Identity
	Name  string
	Level AggregateLevel

	// Child nodes
	Nodes []*CostNode

	// Child aggregates (for hierarchy)
	Children []*CostAggregate

	// Aggregated costs
	TotalMonthly determinism.Money
	TotalHourly  determinism.Money

	// Breakdowns
	ByCategory map[CostCategory]determinism.Money
	ByProvider map[string]determinism.Money
	ByRegion   map[string]determinism.Money

	// Aggregated confidence
	Confidence float64

	// Assumptions count
	AssumptionCount int
}

// AggregateLevel indicates the level of aggregation
type AggregateLevel int

const (
	LevelComponent AggregateLevel = iota // Single component
	LevelResource                         // Single resource
	LevelService                          // Service (e.g., all EC2)
	LevelProvider                         // Provider (e.g., all AWS)
	LevelProject                          // Entire project
)

// String returns the level name
func (l AggregateLevel) String() string {
	switch l {
	case LevelComponent:
		return "component"
	case LevelResource:
		return "resource"
	case LevelService:
		return "service"
	case LevelProvider:
		return "provider"
	case LevelProject:
		return "project"
	default:
		return "unknown"
	}
}

// NewCostAggregate creates a new aggregate
func NewCostAggregate(name string, level AggregateLevel) *CostAggregate {
	return &CostAggregate{
		Name:       name,
		Level:      level,
		Nodes:      []*CostNode{},
		Children:   []*CostAggregate{},
		ByCategory: make(map[CostCategory]determinism.Money),
		ByProvider: make(map[string]determinism.Money),
		ByRegion:   make(map[string]determinism.Money),
		Confidence: 1.0,
	}
}

// AddNode adds a node and updates aggregates
func (a *CostAggregate) AddNode(node *CostNode) {
	a.Nodes = append(a.Nodes, node)
	a.TotalMonthly = a.TotalMonthly.Add(node.TotalMonthly)
	a.TotalHourly = a.TotalHourly.Add(node.TotalHourly)

	// Update category breakdown
	for cat, amount := range node.ByCategory {
		if existing, ok := a.ByCategory[cat]; ok {
			a.ByCategory[cat] = existing.Add(amount)
		} else {
			a.ByCategory[cat] = amount
		}
	}

	// Update provider breakdown
	if existing, ok := a.ByProvider[node.Provider]; ok {
		a.ByProvider[node.Provider] = existing.Add(node.TotalMonthly)
	} else {
		a.ByProvider[node.Provider] = node.TotalMonthly
	}

	// Update region breakdown
	if existing, ok := a.ByRegion[node.Region]; ok {
		a.ByRegion[node.Region] = existing.Add(node.TotalMonthly)
	} else {
		a.ByRegion[node.Region] = node.TotalMonthly
	}

	// Update confidence
	if node.Confidence < a.Confidence {
		a.Confidence = node.Confidence
	}

	// Track assumptions
	a.AssumptionCount += len(node.Assumptions)
}

// AddChild adds a child aggregate
func (a *CostAggregate) AddChild(child *CostAggregate) {
	a.Children = append(a.Children, child)
	a.TotalMonthly = a.TotalMonthly.Add(child.TotalMonthly)
	a.TotalHourly = a.TotalHourly.Add(child.TotalHourly)

	// Merge breakdowns
	for cat, amount := range child.ByCategory {
		if existing, ok := a.ByCategory[cat]; ok {
			a.ByCategory[cat] = existing.Add(amount)
		} else {
			a.ByCategory[cat] = amount
		}
	}

	// Update confidence
	if child.Confidence < a.Confidence {
		a.Confidence = child.Confidence
	}

	a.AssumptionCount += child.AssumptionCount
}

// CostGraph is the complete cost graph
type CostGraph struct {
	// Root aggregate (project level)
	Root *CostAggregate

	// All nodes indexed by instance ID
	NodesByID map[model.InstanceID]*CostNode

	// Aggregates by level
	ByLevel map[AggregateLevel][]*CostAggregate
}

// NewCostGraph creates a new cost graph
func NewCostGraph(projectName string) *CostGraph {
	return &CostGraph{
		Root:      NewCostAggregate(projectName, LevelProject),
		NodesByID: make(map[model.InstanceID]*CostNode),
		ByLevel:   make(map[AggregateLevel][]*CostAggregate),
	}
}

// AddNode adds a cost node to the graph
func (g *CostGraph) AddNode(node *CostNode) {
	g.NodesByID[node.InstanceID] = node
	g.Root.AddNode(node)
}

// GetNode returns a node by instance ID
func (g *CostGraph) GetNode(id model.InstanceID) *CostNode {
	return g.NodesByID[id]
}

// TopCostNodes returns the n highest cost nodes
func (g *CostGraph) TopCostNodes(n int) []*CostNode {
	nodes := make([]*CostNode, 0, len(g.NodesByID))
	for _, node := range g.NodesByID {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].TotalMonthly.Cmp(nodes[j].TotalMonthly) > 0
	})
	if n > len(nodes) {
		n = len(nodes)
	}
	return nodes[:n]
}

// LowConfidenceNodes returns nodes below a confidence threshold
func (g *CostGraph) LowConfidenceNodes(threshold float64) []*CostNode {
	var result []*CostNode
	for _, node := range g.NodesByID {
		if node.Confidence < threshold {
			result = append(result, node)
		}
	}
	return result
}

// BuildServiceAggregates builds service-level aggregates
func (g *CostGraph) BuildServiceAggregates() {
	byService := make(map[string]*CostAggregate)

	for _, node := range g.NodesByID {
		service := extractService(node.ResourceType)
		agg, ok := byService[service]
		if !ok {
			agg = NewCostAggregate(service, LevelService)
			byService[service] = agg
		}
		agg.AddNode(node)
	}

	for _, agg := range byService {
		g.ByLevel[LevelService] = append(g.ByLevel[LevelService], agg)
		g.Root.AddChild(agg)
	}
}

func extractService(resourceType string) string {
	// aws_instance → ec2
	// aws_s3_bucket → s3
	// aws_db_instance → rds
	serviceMap := map[string]string{
		"aws_instance":           "ec2",
		"aws_launch_template":    "ec2",
		"aws_s3_bucket":          "s3",
		"aws_s3_object":          "s3",
		"aws_db_instance":        "rds",
		"aws_rds_cluster":        "rds",
		"aws_lambda_function":    "lambda",
		"aws_ecs_service":        "ecs",
		"aws_ecs_task_definition":"ecs",
		"aws_elasticache_cluster":"elasticache",
		"aws_nat_gateway":        "vpc",
		"aws_lb":                 "elb",
	}
	if service, ok := serviceMap[resourceType]; ok {
		return service
	}
	return "other"
}
