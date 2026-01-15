// Package assets provides AWS asset builders.
package assets

import (
	"context"
	"fmt"

	"terraform-cost/core/asset"
	"terraform-cost/core/types"
)

// baseBuilder provides common functionality for AWS builders
type baseBuilder struct {
	resourceType string
	category     types.AssetCategory
}

func (b *baseBuilder) Provider() types.Provider {
	return types.ProviderAWS
}

func (b *baseBuilder) ResourceType() string {
	return b.resourceType
}

func (b *baseBuilder) Category() types.AssetCategory {
	return b.category
}

// EC2InstanceBuilder builds assets for aws_instance resources
type EC2InstanceBuilder struct {
	baseBuilder
}

// NewEC2InstanceBuilder creates a new EC2 instance builder
func NewEC2InstanceBuilder() asset.Builder {
	return &EC2InstanceBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_instance",
			category:     types.CategoryCompute,
		},
	}
}

// Build converts a raw EC2 instance into an asset
func (b *EC2InstanceBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	instanceType := raw.Attributes.GetString("instance_type")
	if instanceType == "" {
		instanceType = "t3.micro" // Default
	}

	ami := raw.Attributes.GetString("ami")
	az := raw.Attributes.GetString("availability_zone")

	// Extract region from AZ
	region := ""
	if len(az) > 0 {
		region = az[:len(az)-1]
	}

	asset := &types.Asset{
		ID:       fmt.Sprintf("aws_instance.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryCompute,
		Type:     raw.Type,
		Name:     raw.Name,
		Region:   types.Region(region),
		Attributes: types.Attributes{
			"instance_type": {Value: instanceType},
			"ami":           {Value: ami},
			"tenancy":       raw.Attributes["tenancy"],
			"ebs_optimized": raw.Attributes["ebs_optimized"],
			"monitoring":    raw.Attributes["monitoring"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}

	// Add root block device as child
	if rootBlock := raw.Attributes.Get("root_block_device"); rootBlock != nil {
		if rbdList, ok := rootBlock.([]interface{}); ok && len(rbdList) > 0 {
			if rbd, ok := rbdList[0].(map[string]interface{}); ok {
				child := &types.Asset{
					ID:       fmt.Sprintf("%s.root_block_device", asset.ID),
					Address:  types.ResourceAddress(fmt.Sprintf("%s.root_block_device", raw.Address)),
					Provider: types.ProviderAWS,
					Category: types.CategoryStorage,
					Type:     "aws_ebs_volume",
					Name:     "root",
					Parent:   asset,
					Attributes: types.Attributes{
						"volume_type": {Value: getMapString(rbd, "volume_type", "gp3")},
						"volume_size": {Value: getMapInt(rbd, "volume_size", 8)},
						"iops":        {Value: getMapInt(rbd, "iops", 0)},
						"throughput":  {Value: getMapInt(rbd, "throughput", 0)},
						"encrypted":   {Value: getMapBool(rbd, "encrypted", false)},
					},
				}
				asset.Children = append(asset.Children, child)
			}
		}
	}

	// Add additional EBS volumes as children
	if ebsBlocks := raw.Attributes.Get("ebs_block_device"); ebsBlocks != nil {
		if ebsList, ok := ebsBlocks.([]interface{}); ok {
			for i, ebs := range ebsList {
				if ebsMap, ok := ebs.(map[string]interface{}); ok {
					deviceName := getMapString(ebsMap, "device_name", fmt.Sprintf("/dev/sd%c", 'b'+i))
					child := &types.Asset{
						ID:       fmt.Sprintf("%s.ebs_block_device.%d", asset.ID, i),
						Address:  types.ResourceAddress(fmt.Sprintf("%s.ebs_block_device[%d]", raw.Address, i)),
						Provider: types.ProviderAWS,
						Category: types.CategoryStorage,
						Type:     "aws_ebs_volume",
						Name:     deviceName,
						Parent:   asset,
						Attributes: types.Attributes{
							"device_name": {Value: deviceName},
							"volume_type": {Value: getMapString(ebsMap, "volume_type", "gp3")},
							"volume_size": {Value: getMapInt(ebsMap, "volume_size", 8)},
							"iops":        {Value: getMapInt(ebsMap, "iops", 0)},
							"throughput":  {Value: getMapInt(ebsMap, "throughput", 0)},
							"encrypted":   {Value: getMapBool(ebsMap, "encrypted", false)},
						},
					}
					asset.Children = append(asset.Children, child)
				}
			}
		}
	}

	return asset, nil
}

// Helper functions
func extractTags(attrs types.Attributes) map[string]string {
	tags := make(map[string]string)
	if tagsAttr := attrs.Get("tags"); tagsAttr != nil {
		if tagsMap, ok := tagsAttr.(map[string]interface{}); ok {
			for k, v := range tagsMap {
				if s, ok := v.(string); ok {
					tags[k] = s
				}
			}
		}
	}
	return tags
}

func getMapString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getMapInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return defaultVal
}

func getMapBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}
