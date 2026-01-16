// Package confidence - Pessimistic confidence propagation
// Aggregation MUST NOT mask low-confidence components.
// Confidence propagates pessimistically, never optimistically.
package confidence

import (
	"fmt"
	"sort"
)

// PessimisticPropagator ensures confidence degrades, never improves
type PessimisticPropagator struct {
	// Minimum confidence floor
	floor float64

	// Aggregation rules
	rules PropagationRules
}

// PropagationRules define how confidence propagates
type PropagationRules struct {
	// How to combine multiple confidences
	CombineMethod CombineMethod

	// Weight low-confidence items more heavily
	PessimisticBias float64

	// Minimum items to trigger extra penalty
	MinItemsForPenalty int

	// Penalty per additional low-confidence item
	AdditionalPenalty float64
}

// CombineMethod defines how to combine confidences
type CombineMethod int

const (
	// CombineMinimum takes the minimum confidence
	CombineMinimum CombineMethod = iota

	// CombineProduct multiplies confidences
	CombineProduct

	// CombineWeightedMin uses weighted minimum with bias
	CombineWeightedMin

	// CombineHarmonic uses harmonic mean (penalizes outliers)
	CombineHarmonic
)

// NewPessimisticPropagator creates a propagator
func NewPessimisticPropagator() *PessimisticPropagator {
	return &PessimisticPropagator{
		floor: 0.05, // Never go below 5%
		rules: PropagationRules{
			CombineMethod:      CombineWeightedMin,
			PessimisticBias:    1.5,
			MinItemsForPenalty: 3,
			AdditionalPenalty:  0.05,
		},
	}
}

// ConfidenceItem is an item with confidence
type ConfidenceItem struct {
	ID         string
	Confidence float64
	Reason     string
	Category   string
}

// Propagate calculates aggregate confidence pessimistically
func (p *PessimisticPropagator) Propagate(items []ConfidenceItem) *PropagatedConfidence {
	if len(items) == 0 {
		return &PropagatedConfidence{
			Value:       1.0,
			Method:      p.rules.CombineMethod.String(),
			ItemCount:   0,
			LowItems:    []ConfidenceItem{},
			Explanation: "no items to aggregate",
		}
	}

	result := &PropagatedConfidence{
		Method:    p.rules.CombineMethod.String(),
		ItemCount: len(items),
		LowItems:  []ConfidenceItem{},
	}

	// Find low-confidence items (< 80%)
	lowThreshold := 0.8
	for _, item := range items {
		if item.Confidence < lowThreshold {
			result.LowItems = append(result.LowItems, item)
		}
	}

	// Calculate base confidence
	switch p.rules.CombineMethod {
	case CombineMinimum:
		result.Value = p.combineMinimum(items)
	case CombineProduct:
		result.Value = p.combineProduct(items)
	case CombineWeightedMin:
		result.Value = p.combineWeightedMin(items)
	case CombineHarmonic:
		result.Value = p.combineHarmonic(items)
	default:
		result.Value = p.combineMinimum(items)
	}

	// Apply additional penalty for multiple low-confidence items
	if len(result.LowItems) > p.rules.MinItemsForPenalty {
		extraPenalty := float64(len(result.LowItems)-p.rules.MinItemsForPenalty) * p.rules.AdditionalPenalty
		result.Value -= extraPenalty
	}

	// Apply floor
	if result.Value < p.floor {
		result.Value = p.floor
	}

	// Generate explanation
	result.Explanation = p.explain(result, items)

	return result
}

func (p *PessimisticPropagator) combineMinimum(items []ConfidenceItem) float64 {
	min := 1.0
	for _, item := range items {
		if item.Confidence < min {
			min = item.Confidence
		}
	}
	return min
}

func (p *PessimisticPropagator) combineProduct(items []ConfidenceItem) float64 {
	product := 1.0
	for _, item := range items {
		product *= item.Confidence
	}
	return product
}

func (p *PessimisticPropagator) combineWeightedMin(items []ConfidenceItem) float64 {
	if len(items) == 0 {
		return 1.0
	}

	// Sort by confidence
	sorted := make([]ConfidenceItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Confidence < sorted[j].Confidence
	})

	// Apply pessimistic bias to lowest items
	total := 0.0
	weights := 0.0
	for i, item := range sorted {
		// Lower items get higher weight
		weight := p.rules.PessimisticBias
		if i > 0 {
			weight = 1.0
		}
		total += item.Confidence * weight
		weights += weight
	}

	return total / weights
}

func (p *PessimisticPropagator) combineHarmonic(items []ConfidenceItem) float64 {
	if len(items) == 0 {
		return 1.0
	}

	// Harmonic mean: n / sum(1/x)
	sum := 0.0
	for _, item := range items {
		if item.Confidence > 0 {
			sum += 1.0 / item.Confidence
		} else {
			return p.floor // Zero confidence means floor
		}
	}

	return float64(len(items)) / sum
}

func (p *PessimisticPropagator) explain(result *PropagatedConfidence, items []ConfidenceItem) string {
	if len(result.LowItems) == 0 {
		return fmt.Sprintf("All %d components have high confidence", len(items))
	}

	explanation := fmt.Sprintf("%d of %d components have low confidence: ", 
		len(result.LowItems), len(items))
	
	for i, item := range result.LowItems {
		if i > 0 {
			explanation += ", "
		}
		explanation += fmt.Sprintf("%s (%.0f%%)", item.ID, item.Confidence*100)
	}

	return explanation
}

// PropagatedConfidence is the result of propagation
type PropagatedConfidence struct {
	Value       float64
	Method      string
	ItemCount   int
	LowItems    []ConfidenceItem
	Explanation string
}

// String returns the combine method name
func (m CombineMethod) String() string {
	switch m {
	case CombineMinimum:
		return "minimum"
	case CombineProduct:
		return "product"
	case CombineWeightedMin:
		return "weighted_min"
	case CombineHarmonic:
		return "harmonic"
	default:
		return "unknown"
	}
}

// ConfidenceAggregator aggregates confidence with full visibility
type ConfidenceAggregator struct {
	propagator *PessimisticPropagator
	items      []ConfidenceItem
	result     *PropagatedConfidence
}

// NewConfidenceAggregator creates an aggregator
func NewConfidenceAggregator() *ConfidenceAggregator {
	return &ConfidenceAggregator{
		propagator: NewPessimisticPropagator(),
		items:      []ConfidenceItem{},
	}
}

// Add adds an item
func (a *ConfidenceAggregator) Add(id string, confidence float64, reason, category string) {
	a.items = append(a.items, ConfidenceItem{
		ID:         id,
		Confidence: confidence,
		Reason:     reason,
		Category:   category,
	})
	a.result = nil // Invalidate cached result
}

// Result returns the propagated confidence
func (a *ConfidenceAggregator) Result() *PropagatedConfidence {
	if a.result == nil {
		a.result = a.propagator.Propagate(a.items)
	}
	return a.result
}

// HasLowConfidence returns true if any item is below threshold
func (a *ConfidenceAggregator) HasLowConfidence(threshold float64) bool {
	for _, item := range a.items {
		if item.Confidence < threshold {
			return true
		}
	}
	return false
}

// LowestConfidence returns the item with lowest confidence
func (a *ConfidenceAggregator) LowestConfidence() *ConfidenceItem {
	if len(a.items) == 0 {
		return nil
	}

	lowest := &a.items[0]
	for i := range a.items {
		if a.items[i].Confidence < lowest.Confidence {
			lowest = &a.items[i]
		}
	}
	return lowest
}

// LowConfidenceWarning generates a warning for low confidence
func (a *ConfidenceAggregator) LowConfidenceWarning() string {
	result := a.Result()
	if result.Value >= 0.8 {
		return ""
	}

	return fmt.Sprintf("⚠️ Low confidence (%.0f%%): %s", 
		result.Value*100, result.Explanation)
}
