// Package confidence - Pessimistic confidence propagation
// Aggregate confidence = MIN(child confidence)
// This ensures low-confidence components are never hidden.
package confidence

// AggregateConfidence returns the minimum confidence (pessimistic)
// This is the REQUIRED behavior for all aggregations.
func AggregateConfidence(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	min := 1.0
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

// AggregateWithWeights returns weighted minimum
// Still pessimistic, but considers weight
func AggregateWithWeights(values []WeightedConfidence) float64 {
	if len(values) == 0 {
		return 0.0
	}

	min := 1.0
	for _, wc := range values {
		// Weight affects how much this drags down the aggregate
		effective := wc.Confidence
		if wc.Weight > 0 && wc.Confidence < 1.0 {
			// Heavier weights have more impact
			penalty := (1.0 - wc.Confidence) * wc.Weight
			effective = 1.0 - penalty
		}
		if effective < min {
			min = effective
		}
	}
	return min
}

// WeightedConfidence is a confidence value with weight
type WeightedConfidence struct {
	Confidence float64
	Weight     float64 // 0.0 to 1.0
}

// ConfidenceTracker tracks confidence across operations
type ConfidenceTracker struct {
	values []float64
	min    float64
}

// NewConfidenceTracker creates a tracker
func NewConfidenceTracker() *ConfidenceTracker {
	return &ConfidenceTracker{
		values: []float64{},
		min:    1.0,
	}
}

// Add adds a confidence value
func (t *ConfidenceTracker) Add(confidence float64) {
	t.values = append(t.values, confidence)
	if confidence < t.min {
		t.min = confidence
	}
}

// AddWithReason adds a confidence value with reason
func (t *ConfidenceTracker) AddWithReason(confidence float64, reason string) {
	t.Add(confidence)
	// Could track reasons if needed
}

// Min returns the minimum confidence
func (t *ConfidenceTracker) Min() float64 {
	return t.min
}

// Apply applies a degradation factor
func (t *ConfidenceTracker) Apply(factor string, reason string) {
	// Apply a default degradation
	t.Add(0.9)
}

// Average returns the average (NOT recommended for production)
func (t *ConfidenceTracker) Average() float64 {
	if len(t.values) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, v := range t.values {
		sum += v
	}
	return sum / float64(len(t.values))
}

// IsDegraded returns true if confidence is below threshold
func (t *ConfidenceTracker) IsDegraded(threshold float64) bool {
	return t.min < threshold
}

// ConfidenceImpact represents what impacts confidence
type ConfidenceImpact struct {
	Source     string  // What caused the impact
	Impact     float64 // How much it reduces confidence (0.0-1.0)
	Reason     string
	IsBlocking bool // If true in strict mode, blocks estimation
}

// ApplyImpacts applies confidence impacts
func ApplyImpacts(base float64, impacts []ConfidenceImpact) float64 {
	result := base
	for _, impact := range impacts {
		result *= (1.0 - impact.Impact)
	}
	if result < 0 {
		result = 0
	}
	return result
}

// REQUIRED confidence thresholds
const (
	ConfidenceHigh   = 0.9
	ConfidenceMedium = 0.7
	ConfidenceLow    = 0.5
	ConfidenceNone   = 0.0
)

// ConfidenceLevel returns a human-readable level
func ConfidenceLevel(confidence float64) string {
	switch {
	case confidence >= ConfidenceHigh:
		return "high"
	case confidence >= ConfidenceMedium:
		return "medium"
	case confidence >= ConfidenceLow:
		return "low"
	default:
		return "unknown"
	}
}
