package expansion

import (
	"testing"

	"terraform-cost/core/expression"
	"terraform-cost/core/types"
)

// TestCountExpansion tests count-based expansion behavior
func TestCountExpansion(t *testing.T) {
	expander := NewExpander()
	ctx := expression.NewContext()

	tests := []struct {
		name          string
		countValue    interface{}
		expectedCount int
		isKnown       bool
	}{
		{
			name:          "count=0 produces nothing",
			countValue:    0,
			expectedCount: 0,
			isKnown:       true,
		},
		{
			name:          "count=1 produces one instance",
			countValue:    1,
			expectedCount: 1,
			isKnown:       true,
		},
		{
			name:          "count=3 produces three instances",
			countValue:    3,
			expectedCount: 3,
			isKnown:       true,
		},
		{
			name:          "count=5 produces five instances",
			countValue:    5,
			expectedCount: 5,
			isKnown:       true,
		},
		{
			name:          "unknown count produces default with warning",
			countValue:    expression.Unknown(),
			expectedCount: 1,
			isKnown:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &types.Asset{
				Address: "aws_instance.test",
				Type:    "aws_instance",
				Name:    "test",
				Attributes: types.Attributes{
					"count": {Value: tt.countValue},
				},
			}

			instances, err := expander.Expand(asset, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(instances) != tt.expectedCount {
				t.Errorf("expected %d instances, got %d", tt.expectedCount, len(instances))
			}

			// Verify instance addresses
			for i, inst := range instances {
				expectedKey := i
				if inst.Key.NumValue != expectedKey {
					t.Errorf("instance %d: expected key %d, got %d", i, expectedKey, inst.Key.NumValue)
				}

				if inst.Metadata.IsKnown != tt.isKnown {
					t.Errorf("instance %d: expected isKnown=%v, got %v", i, tt.isKnown, inst.Metadata.IsKnown)
				}

				if !tt.isKnown && inst.Metadata.Warning == "" {
					t.Errorf("instance %d: expected warning for unknown count", i)
				}
			}
		})
	}
}

// TestForEachExpansion tests for_each-based expansion behavior
func TestForEachExpansion(t *testing.T) {
	expander := NewExpander()
	ctx := expression.NewContext()

	tests := []struct {
		name         string
		forEachValue interface{}
		expectedKeys []string
		isKnown      bool
	}{
		{
			name:         "empty map produces nothing",
			forEachValue: map[string]interface{}{},
			expectedKeys: []string{},
			isKnown:      true,
		},
		{
			name: "map with one key",
			forEachValue: map[string]interface{}{
				"web": "value1",
			},
			expectedKeys: []string{"web"},
			isKnown:      true,
		},
		{
			name: "map with multiple keys",
			forEachValue: map[string]interface{}{
				"a": 1,
				"b": 2,
				"c": 3,
			},
			expectedKeys: []string{"a", "b", "c"}, // sorted
			isKnown:      true,
		},
		{
			name:         "set as list of strings",
			forEachValue: []interface{}{"alpha", "beta", "gamma"},
			expectedKeys: []string{"alpha", "beta", "gamma"},
			isKnown:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &types.Asset{
				Address: "aws_instance.multi",
				Type:    "aws_instance",
				Name:    "multi",
				Attributes: types.Attributes{
					"for_each": {Value: tt.forEachValue},
				},
			}

			instances, err := expander.Expand(asset, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(instances) != len(tt.expectedKeys) {
				t.Errorf("expected %d instances, got %d", len(tt.expectedKeys), len(instances))
			}

			// Verify instance keys (they should be sorted)
			for i, inst := range instances {
				if i >= len(tt.expectedKeys) {
					break
				}
				expectedKey := tt.expectedKeys[i]
				if inst.Key.StrValue != expectedKey {
					t.Errorf("instance %d: expected key %q, got %q", i, expectedKey, inst.Key.StrValue)
				}

				if inst.Key.Type != KeyTypeString {
					t.Errorf("instance %d: expected string key type, got %v", i, inst.Key.Type)
				}
			}
		})
	}
}

// TestNoExpansion tests that assets without count/for_each produce single instance
func TestNoExpansion(t *testing.T) {
	expander := NewExpander()
	ctx := expression.NewContext()

	asset := &types.Asset{
		Address: "aws_instance.single",
		Type:    "aws_instance",
		Name:    "single",
		Attributes: types.Attributes{
			"instance_type": {Value: "t3.micro"},
		},
	}

	instances, err := expander.Expand(asset, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(instances))
	}

	if instances[0].Key.Type != KeyTypeNone {
		t.Errorf("expected no key type, got %v", instances[0].Key.Type)
	}

	if instances[0].Address != asset.Address {
		t.Errorf("expected address %s, got %s", asset.Address, instances[0].Address)
	}

	if instances[0].Metadata.ExpansionType != ExpansionNone {
		t.Errorf("expected ExpansionNone, got %v", instances[0].Metadata.ExpansionType)
	}
}

// TestInstanceAddresses tests that addresses are correctly formatted
func TestInstanceAddresses(t *testing.T) {
	expander := NewExpander()
	ctx := expression.NewContext()

	// Test count addresses
	countAsset := &types.Asset{
		Address:    "aws_instance.counted",
		Type:       "aws_instance",
		Name:       "counted",
		Attributes: types.Attributes{"count": {Value: 2}},
	}

	countInstances, _ := expander.Expand(countAsset, ctx)
	expectedAddrs := []types.ResourceAddress{
		"aws_instance.counted[0]",
		"aws_instance.counted[1]",
	}

	for i, inst := range countInstances {
		if inst.Address != expectedAddrs[i] {
			t.Errorf("count instance %d: expected address %s, got %s", i, expectedAddrs[i], inst.Address)
		}
	}

	// Test for_each addresses
	forEachAsset := &types.Asset{
		Address: "aws_instance.named",
		Type:    "aws_instance",
		Name:    "named",
		Attributes: types.Attributes{
			"for_each": {Value: map[string]interface{}{"web": 1, "api": 2}},
		},
	}

	forEachInstances, _ := expander.Expand(forEachAsset, ctx)

	// Check that addresses are quoted correctly
	for _, inst := range forEachInstances {
		addrStr := string(inst.Address)
		if inst.Key.StrValue == "api" {
			if addrStr != `aws_instance.named["api"]` {
				t.Errorf("expected address aws_instance.named[\"api\"], got %s", addrStr)
			}
		}
		if inst.Key.StrValue == "web" {
			if addrStr != `aws_instance.named["web"]` {
				t.Errorf("expected address aws_instance.named[\"web\"], got %s", addrStr)
			}
		}
	}
}

// TestExpandedGraph tests the expanded graph construction
func TestExpandedGraph(t *testing.T) {
	instances := []*AssetInstance{
		{
			Address: "aws_instance.test[0]",
			Key:     InstanceKey{Type: KeyTypeInt, NumValue: 0},
			Metadata: InstanceMetadata{
				OriginalAddress: "aws_instance.test",
				ExpansionType:   ExpansionCount,
			},
		},
		{
			Address: "aws_instance.test[1]",
			Key:     InstanceKey{Type: KeyTypeInt, NumValue: 1},
			Metadata: InstanceMetadata{
				OriginalAddress: "aws_instance.test",
				ExpansionType:   ExpansionCount,
			},
		},
		{
			Address: "aws_s3_bucket.single",
			Key:     InstanceKey{Type: KeyTypeNone},
			Metadata: InstanceMetadata{
				OriginalAddress: "aws_s3_bucket.single",
				ExpansionType:   ExpansionNone,
			},
		},
	}

	graph := NewExpandedGraph(instances)

	if len(graph.Instances) != 3 {
		t.Errorf("expected 3 instances, got %d", len(graph.Instances))
	}

	// Test ByAddress lookup
	inst, ok := graph.ByAddress["aws_instance.test[0]"]
	if !ok {
		t.Error("ByAddress lookup failed for aws_instance.test[0]")
	}
	if inst.Key.NumValue != 0 {
		t.Errorf("expected key value 0, got %d", inst.Key.NumValue)
	}

	// Test ByBaseAddress grouping
	baseInstances := graph.ByBaseAddress["aws_instance.test"]
	if len(baseInstances) != 2 {
		t.Errorf("expected 2 instances for base address, got %d", len(baseInstances))
	}
}
