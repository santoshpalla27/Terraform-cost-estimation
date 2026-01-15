// Package types - Asset domain types
package types

// RawAsset represents a parsed infrastructure resource before graph construction.
// This is the output of scanners - no pricing or cost information here.
type RawAsset struct {
	// Address is the Terraform resource address
	Address ResourceAddress `json:"address"`

	// Provider is the cloud provider (aws, azure, gcp)
	Provider Provider `json:"provider"`

	// Type is the resource type (e.g., "aws_instance", "aws_s3_bucket")
	Type string `json:"type"`

	// Name is the resource name from the Terraform configuration
	Name string `json:"name"`

	// Attributes contains all resource attributes
	Attributes Attributes `json:"attributes"`

	// Module is the module path (empty for root module)
	Module string `json:"module,omitempty"`

	// Count is the count value if using count meta-argument
	Count int `json:"count,omitempty"`

	// ForEach contains the for_each keys if using for_each meta-argument
	ForEach []string `json:"for_each,omitempty"`

	// IsDataSource indicates if this is a data source, not a resource
	IsDataSource bool `json:"is_data_source"`

	// SourceFile is the file where this resource is defined
	SourceFile string `json:"source_file,omitempty"`

	// SourceLine is the line number in the source file
	SourceLine int `json:"source_line,omitempty"`
}

// Asset represents a node in the Asset Graph.
// This is the normalized, provider-agnostic representation of infrastructure.
type Asset struct {
	// ID is a unique identifier for this asset
	ID string `json:"id"`

	// Address is the original Terraform resource address
	Address ResourceAddress `json:"address"`

	// Provider is the cloud provider
	Provider Provider `json:"provider"`

	// Category groups assets by function (compute, storage, network, etc.)
	Category AssetCategory `json:"category"`

	// Type is the resource type
	Type string `json:"type"`

	// Name is the resource name
	Name string `json:"name"`

	// Attributes contains normalized resource attributes
	Attributes Attributes `json:"attributes"`

	// Children contains child assets (e.g., EBS volumes attached to an EC2 instance)
	Children []*Asset `json:"children,omitempty"`

	// Parent is the parent asset (nil for root-level assets)
	Parent *Asset `json:"-"`

	// Dependencies lists assets this asset depends on
	Dependencies []*Asset `json:"-"`

	// Metadata contains additional information about the asset
	Metadata AssetMetadata `json:"metadata"`

	// Region is the deployment region
	Region Region `json:"region,omitempty"`

	// Tags are resource tags
	Tags map[string]string `json:"tags,omitempty"`
}

// AssetCategory groups assets by their primary function
type AssetCategory string

const (
	CategoryCompute    AssetCategory = "compute"
	CategoryStorage    AssetCategory = "storage"
	CategoryNetwork    AssetCategory = "network"
	CategoryDatabase   AssetCategory = "database"
	CategoryContainer  AssetCategory = "container"
	CategoryServerless AssetCategory = "serverless"
	CategorySecurity   AssetCategory = "security"
	CategoryMonitoring AssetCategory = "monitoring"
	CategoryAI         AssetCategory = "ai_ml"
	CategoryOther      AssetCategory = "other"
)

// String returns the string representation
func (c AssetCategory) String() string {
	return string(c)
}

// AssetMetadata contains asset-specific metadata
type AssetMetadata struct {
	// Source is the file path where the asset is defined
	Source string `json:"source,omitempty"`

	// Line is the line number in the source file
	Line int `json:"line,omitempty"`

	// IsDataSource indicates if this originated from a data source
	IsDataSource bool `json:"is_data_source"`

	// ModulePath is the module path
	ModulePath string `json:"module_path,omitempty"`

	// Index is the count or for_each index
	Index string `json:"index,omitempty"`
}

// AssetGraph represents the complete infrastructure as a directed acyclic graph
type AssetGraph struct {
	// Roots contains top-level assets (no parent)
	Roots []*Asset `json:"roots"`

	// ByID provides O(1) lookup by asset ID
	ByID map[string]*Asset `json:"-"`

	// ByAddress provides lookup by Terraform address
	ByAddress map[ResourceAddress]*Asset `json:"-"`

	// ByProvider groups assets by cloud provider
	ByProvider map[Provider][]*Asset `json:"-"`

	// ByCategory groups assets by category
	ByCategory map[AssetCategory][]*Asset `json:"-"`

	// Metadata contains graph-level metadata
	Metadata GraphMetadata `json:"metadata"`
}

// GraphMetadata contains metadata about the asset graph
type GraphMetadata struct {
	// TotalAssets is the total count of assets
	TotalAssets int `json:"total_assets"`

	// Providers lists all providers in the graph
	Providers []Provider `json:"providers"`

	// Modules lists all module paths
	Modules []string `json:"modules,omitempty"`
}

// NewAssetGraph creates a new empty asset graph
func NewAssetGraph() *AssetGraph {
	return &AssetGraph{
		Roots:      make([]*Asset, 0),
		ByID:       make(map[string]*Asset),
		ByAddress:  make(map[ResourceAddress]*Asset),
		ByProvider: make(map[Provider][]*Asset),
		ByCategory: make(map[AssetCategory][]*Asset),
	}
}

// Add adds an asset to the graph
func (g *AssetGraph) Add(asset *Asset) {
	g.ByID[asset.ID] = asset
	g.ByAddress[asset.Address] = asset
	g.ByProvider[asset.Provider] = append(g.ByProvider[asset.Provider], asset)
	g.ByCategory[asset.Category] = append(g.ByCategory[asset.Category], asset)

	if asset.Parent == nil {
		g.Roots = append(g.Roots, asset)
	}

	g.Metadata.TotalAssets++
}

// Walk traverses all assets in the graph, calling fn for each
func (g *AssetGraph) Walk(fn func(*Asset) error) error {
	for _, root := range g.Roots {
		if err := walkAsset(root, fn); err != nil {
			return err
		}
	}
	return nil
}

func walkAsset(asset *Asset, fn func(*Asset) error) error {
	if err := fn(asset); err != nil {
		return err
	}
	for _, child := range asset.Children {
		if err := walkAsset(child, fn); err != nil {
			return err
		}
	}
	return nil
}
