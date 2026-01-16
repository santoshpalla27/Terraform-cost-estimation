// Package confidence provides aggressive confidence propagation rules.
// Every unknown COMPOUNDS confidence loss. No hiding uncertainty.
package confidence

import (
	"fmt"
	"math"
	"strings"

	"terraform-cost/core/determinism"
)

// DecayRule defines how confidence decays
type DecayRule struct {
	Name        string
	BaseFactor  float64 // Base multiplier (e.g., 0.8 = 20% reduction)
	Compounds   bool    // Does this compound with other decays?
	MinConfidence float64 // Floor (never go below this)
}

// StandardDecayRules are the default decay rules
var StandardDecayRules = map[string]*DecayRule{
	"unknown_value": {
		Name:        "unknown_value",
		BaseFactor:  0.7, // Each unknown reduces by 30%
		Compounds:   true,
		MinConfidence: 0.1,
	},
	"unknown_count": {
		Name:        "unknown_count",
		BaseFactor:  0.5, // Unknown count is severe - 50% reduction
		Compounds:   true,
		MinConfidence: 0.1,
	},
	"unknown_for_each": {
		Name:        "unknown_for_each",
		BaseFactor:  0.5, // Unknown for_each is severe
		Compounds:   true,
		MinConfidence: 0.1,
	},
	"unknown_usage": {
		Name:        "unknown_usage",
		BaseFactor:  0.6, // Unknown usage is 40% reduction
		Compounds:   true,
		MinConfidence: 0.2,
	},
	"missing_rate": {
		Name:        "missing_rate",
		BaseFactor:  0.3, // Missing rate is very severe
		Compounds:   true,
		MinConfidence: 0.05,
	},
	"default_usage": {
		Name:        "default_usage",
		BaseFactor:  0.8, // Using defaults is 20% reduction
		Compounds:   true,
		MinConfidence: 0.3,
	},
	"stale_snapshot": {
		Name:        "stale_snapshot",
		BaseFactor:  0.9, // Stale snapshot is 10% reduction
		Compounds:   false,
		MinConfidence: 0.5,
	},
}

// ConfidenceTracker tracks confidence with full reasoning
type ConfidenceTracker struct {
	initialConfidence float64
	currentConfidence float64
	factors          []ConfidenceDecay
	compoundCount    int
}

// ConfidenceDecay records a single confidence reduction
type ConfidenceDecay struct {
	Rule        string
	Reason      string
	Factor      float64
	AppliedAt   float64 // Confidence before this decay
	ResultedIn  float64 // Confidence after this decay
	WasCompounded bool
}

// NewConfidenceTracker creates a tracker starting at full confidence
func NewConfidenceTracker() *ConfidenceTracker {
	return &ConfidenceTracker{
		initialConfidence: 1.0,
		currentConfidence: 1.0,
		factors:           []ConfidenceDecay{},
	}
}

// Apply applies a decay rule
func (t *ConfidenceTracker) Apply(ruleName, reason string) {
	rule, ok := StandardDecayRules[ruleName]
	if !ok {
		// Unknown rule - use moderate decay
		rule = &DecayRule{
			Name:       ruleName,
			BaseFactor: 0.8,
			Compounds:  true,
			MinConfidence: 0.1,
		}
	}

	before := t.currentConfidence

	// Calculate decay factor
	factor := rule.BaseFactor

	// Compound decay: each subsequent unknown has increasing impact
	if rule.Compounds && t.compoundCount > 0 {
		// Each compound increases decay: 0.7 → 0.6 → 0.5 → ...
		compoundPenalty := math.Pow(0.9, float64(t.compoundCount))
		factor *= compoundPenalty
	}

	// Apply decay
	t.currentConfidence *= factor

	// Enforce floor
	if t.currentConfidence < rule.MinConfidence {
		t.currentConfidence = rule.MinConfidence
	}

	// Record
	t.factors = append(t.factors, ConfidenceDecay{
		Rule:        ruleName,
		Reason:      reason,
		Factor:      factor,
		AppliedAt:   before,
		ResultedIn:  t.currentConfidence,
		WasCompounded: rule.Compounds && t.compoundCount > 0,
	})

	if rule.Compounds {
		t.compoundCount++
	}
}

// ApplyMultiple applies multiple decays for compound unknown scenarios
func (t *ConfidenceTracker) ApplyMultiple(rules []string, reason string) {
	for _, rule := range rules {
		t.Apply(rule, reason)
	}
}

// Current returns the current confidence
func (t *ConfidenceTracker) Current() float64 {
	return t.currentConfidence
}

// Factors returns all decay factors
func (t *ConfidenceTracker) Factors() []ConfidenceDecay {
	return t.factors
}

// TotalDecay returns how much confidence was lost
func (t *ConfidenceTracker) TotalDecay() float64 {
	return t.initialConfidence - t.currentConfidence
}

// Level returns human-readable confidence level
func (t *ConfidenceTracker) Level() string {
	switch {
	case t.currentConfidence >= 0.9:
		return "high"
	case t.currentConfidence >= 0.7:
		return "medium"
	case t.currentConfidence >= 0.5:
		return "low"
	case t.currentConfidence >= 0.3:
		return "very_low"
	default:
		return "unreliable"
	}
}

// Explain returns human-readable explanation
func (t *ConfidenceTracker) Explain() string {
	if len(t.factors) == 0 {
		return "Full confidence - no uncertainties"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Confidence: %.0f%% (%s)\n", t.currentConfidence*100, t.Level()))
	sb.WriteString("Decay factors:\n")

	for i, f := range t.factors {
		compound := ""
		if f.WasCompounded {
			compound = " [compounded]"
		}
		sb.WriteString(fmt.Sprintf("  %d. %s: %.0f%% → %.0f%% (%s)%s\n",
			i+1, f.Rule, f.AppliedAt*100, f.ResultedIn*100, f.Reason, compound))
	}

	return sb.String()
}

// ConfidenceBoundValue is a value with confidence metadata
type ConfidenceBoundValue struct {
	Value      interface{}
	Confidence float64
	Tracker    *ConfidenceTracker
	IsUnknown  bool
}

// NewKnownValue creates a known value with full confidence
func NewKnownValue(v interface{}) *ConfidenceBoundValue {
	return &ConfidenceBoundValue{
		Value:      v,
		Confidence: 1.0,
		Tracker:    NewConfidenceTracker(),
		IsUnknown:  false,
	}
}

// NewUnknownValue creates an unknown value with degraded confidence
func NewUnknownValue(reason string) *ConfidenceBoundValue {
	tracker := NewConfidenceTracker()
	tracker.Apply("unknown_value", reason)
	return &ConfidenceBoundValue{
		Value:      nil,
		Confidence: tracker.Current(),
		Tracker:    tracker,
		IsUnknown:  true,
	}
}

// ConfidenceAwareMath does arithmetic that propagates confidence
type ConfidenceAwareMath struct{}

// Multiply multiplies values, compounding confidence
func (ConfidenceAwareMath) Multiply(a, b *ConfidenceBoundValue) *ConfidenceBoundValue {
	// If either is unknown, result is unknown with compounded confidence
	if a.IsUnknown || b.IsUnknown {
		tracker := NewConfidenceTracker()

		if a.IsUnknown {
			tracker.Apply("unknown_value", "left operand unknown")
		}
		if b.IsUnknown {
			tracker.Apply("unknown_value", "right operand unknown")
		}

		return &ConfidenceBoundValue{
			Value:      nil,
			Confidence: tracker.Current(),
			Tracker:    tracker,
			IsUnknown:  true,
		}
	}

	// Both known - combine confidences multiplicatively
	aVal, aOk := toFloat64(a.Value)
	bVal, bOk := toFloat64(b.Value)

	if !aOk || !bOk {
		tracker := NewConfidenceTracker()
		tracker.Apply("unknown_value", "non-numeric operand")
		return &ConfidenceBoundValue{
			Value:      nil,
			Confidence: tracker.Current(),
			Tracker:    tracker,
			IsUnknown:  true,
		}
	}

	// Result confidence is product of input confidences
	resultConfidence := a.Confidence * b.Confidence

	return &ConfidenceBoundValue{
		Value:      aVal * bVal,
		Confidence: resultConfidence,
		Tracker:    combineTrackers(a.Tracker, b.Tracker),
		IsUnknown:  false,
	}
}

// Add adds values, using minimum confidence
func (ConfidenceAwareMath) Add(a, b *ConfidenceBoundValue) *ConfidenceBoundValue {
	if a.IsUnknown && b.IsUnknown {
		tracker := NewConfidenceTracker()
		tracker.ApplyMultiple([]string{"unknown_value", "unknown_value"}, "both operands unknown")
		return &ConfidenceBoundValue{
			Value:      nil,
			Confidence: tracker.Current(),
			Tracker:    tracker,
			IsUnknown:  true,
		}
	}

	if a.IsUnknown {
		// Use b's value but with reduced confidence
		tracker := NewConfidenceTracker()
		tracker.Apply("unknown_value", "left operand unknown")
		return &ConfidenceBoundValue{
			Value:      b.Value,
			Confidence: math.Min(b.Confidence, tracker.Current()),
			Tracker:    tracker,
			IsUnknown:  false,
		}
	}

	if b.IsUnknown {
		tracker := NewConfidenceTracker()
		tracker.Apply("unknown_value", "right operand unknown")
		return &ConfidenceBoundValue{
			Value:      a.Value,
			Confidence: math.Min(a.Confidence, tracker.Current()),
			Tracker:    tracker,
			IsUnknown:  false,
		}
	}

	// Both known
	aVal, aOk := toFloat64(a.Value)
	bVal, bOk := toFloat64(b.Value)

	if !aOk || !bOk {
		return NewUnknownValue("non-numeric operand")
	}

	return &ConfidenceBoundValue{
		Value:      aVal + bVal,
		Confidence: math.Min(a.Confidence, b.Confidence),
		Tracker:    combineTrackers(a.Tracker, b.Tracker),
		IsUnknown:  false,
	}
}

// ConfidenceBoundMoney is money with confidence
type ConfidenceBoundMoney struct {
	Amount     determinism.Money
	Confidence float64
	Tracker    *ConfidenceTracker
}

// NewConfidenceBoundMoney creates money with full confidence
func NewConfidenceBoundMoney(amount determinism.Money) *ConfidenceBoundMoney {
	return &ConfidenceBoundMoney{
		Amount:     amount,
		Confidence: 1.0,
		Tracker:    NewConfidenceTracker(),
	}
}

// Degrade reduces confidence with a reason
func (m *ConfidenceBoundMoney) Degrade(rule, reason string) {
	m.Tracker.Apply(rule, reason)
	m.Confidence = m.Tracker.Current()
}

// Add adds two money values, taking minimum confidence
func (m *ConfidenceBoundMoney) Add(other *ConfidenceBoundMoney) *ConfidenceBoundMoney {
	return &ConfidenceBoundMoney{
		Amount:     m.Amount.Add(other.Amount),
		Confidence: math.Min(m.Confidence, other.Confidence),
		Tracker:    combineTrackers(m.Tracker, other.Tracker),
	}
}

// Multiply multiplies money by a factor, compounding confidence
func (m *ConfidenceBoundMoney) Multiply(factor *ConfidenceBoundValue) *ConfidenceBoundMoney {
	if factor.IsUnknown {
		tracker := NewConfidenceTracker()
		tracker.Apply("unknown_usage", "multiplier unknown")
		return &ConfidenceBoundMoney{
			Amount:     m.Amount, // Keep original as estimate
			Confidence: m.Confidence * tracker.Current(),
			Tracker:    combineTrackers(m.Tracker, tracker),
		}
	}

	factorVal, ok := toFloat64(factor.Value)
	if !ok {
		factorVal = 1.0 // Default to 1 if can't convert
	}

	return &ConfidenceBoundMoney{
		Amount:     m.Amount.MulFloat(factorVal),
		Confidence: m.Confidence * factor.Confidence,
		Tracker:    combineTrackers(m.Tracker, factor.Tracker),
	}
}

// Explain returns the confidence explanation
func (m *ConfidenceBoundMoney) Explain() string {
	return fmt.Sprintf("$%s (confidence: %.0f%% - %s)\n%s",
		m.Amount.String(), m.Confidence*100, m.Tracker.Level(), m.Tracker.Explain())
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func combineTrackers(a, b *ConfidenceTracker) *ConfidenceTracker {
	combined := NewConfidenceTracker()
	if a != nil {
		for _, f := range a.factors {
			combined.factors = append(combined.factors, f)
		}
		combined.compoundCount += a.compoundCount
	}
	if b != nil {
		for _, f := range b.factors {
			combined.factors = append(combined.factors, f)
		}
		combined.compoundCount += b.compoundCount
	}
	
	// Recalculate current confidence
	combined.currentConfidence = 1.0
	for _, f := range combined.factors {
		combined.currentConfidence *= f.Factor
	}
	if combined.currentConfidence < 0.05 {
		combined.currentConfidence = 0.05
	}
	
	return combined
}
