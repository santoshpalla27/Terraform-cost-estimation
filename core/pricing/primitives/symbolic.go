// Package primitives - Symbolic cost helpers
// Used when cost cannot be determined
package primitives

// Symbolic creates a symbolic (unknown) cost unit
func Symbolic(name string, reason string) CostUnit {
	return CostUnit{
		Name:           name,
		IsSymbolic:     true,
		SymbolicReason: reason,
		Confidence:     0,
	}
}

// SymbolicWithEstimate creates a symbolic cost with an upper bound estimate
func SymbolicWithEstimate(name string, reason string, estimatedMax float64) CostUnit {
	return CostUnit{
		Name:           name,
		Measure:        "estimated",
		Quantity:       estimatedMax,
		IsSymbolic:     true,
		SymbolicReason: reason,
		Confidence:     0,
	}
}

// ZeroCost creates a cost unit that is explicitly zero
// Used for indirect/free resources
func ZeroCost(name string, reason string) CostUnit {
	return CostUnit{
		Name:       name,
		Measure:    "none",
		Quantity:   0,
		Confidence: 1.0,
		RateKey: RateKey{
			Attributes: map[string]string{
				"reason": reason,
			},
		},
	}
}

// Unsupported creates a cost unit for unsupported resources
func Unsupported(resourceType string) CostUnit {
	return CostUnit{
		Name:           "unsupported",
		IsSymbolic:     true,
		SymbolicReason: "resource type " + resourceType + " is not supported",
		Confidence:     0,
	}
}
