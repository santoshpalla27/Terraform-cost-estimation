// Package explanation - API response types for explanations
// Safe for JSON serialization and UI consumption
package explanation

// ExplanationResponse is the API response for cost explanations
type ExplanationResponse struct {
	ResourceExplanations []ResourceExplanation `json:"resource_explanations"`
	SymbolicSummary      *SymbolicSummary      `json:"symbolic_summary,omitempty"`
	DiffNarratives       []DiffNarrative       `json:"diff_narratives,omitempty"`
}

// ResourceExplanation is the API-safe explanation for a resource
type ResourceExplanation struct {
	Address      string              `json:"address"`
	ResourceType string              `json:"resource_type"`
	MonthlyCost  float64             `json:"monthly_cost"`
	IsSymbolic   bool                `json:"is_symbolic"`
	Components   []ComponentExplanation `json:"components"`
}

// ComponentExplanation is the API-safe explanation for a cost component
type ComponentExplanation struct {
	Name       string            `json:"name"`
	Cost       float64           `json:"cost"`
	Quantity   float64           `json:"quantity"`
	Unit       string            `json:"unit"`
	Formula    string            `json:"formula,omitempty"`
	Inputs     map[string]string `json:"inputs,omitempty"`
	Confidence float64           `json:"confidence"`
	IsSymbolic bool              `json:"is_symbolic"`
	SymbolicReason string        `json:"symbolic_reason,omitempty"`
}

// SymbolicSummary summarizes all symbolic costs
type SymbolicSummary struct {
	Count    int                    `json:"count"`
	Reasons  map[string]int         `json:"reasons"`
	Details  []SymbolicDetail       `json:"details"`
}

// SymbolicDetail is a single symbolic cost detail
type SymbolicDetail struct {
	Resource    string   `json:"resource"`
	Component   string   `json:"component"`
	Reason      string   `json:"reason"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// BuildExplanationResponse builds an API response from explanations
func BuildExplanationResponse(explanations []*CostExplanation) ExplanationResponse {
	response := ExplanationResponse{
		ResourceExplanations: make([]ResourceExplanation, 0),
	}
	
	// Group by resource
	grouped := make(map[string][]ComponentExplanation)
	symbolicReasons := make(map[string]int)
	var symbolicDetails []SymbolicDetail
	
	for _, exp := range explanations {
		comp := ComponentExplanation{
			Name:       exp.CostUnit,
			Formula:    exp.Formula,
			Confidence: exp.Confidence,
			IsSymbolic: exp.IsSymbolic,
			SymbolicReason: exp.SymbolicReason,
			Inputs:     make(map[string]string),
		}
		
		for _, input := range exp.Inputs {
			comp.Inputs[input.Name] = input.Value
		}
		
		grouped[exp.Resource] = append(grouped[exp.Resource], comp)
		
		if exp.IsSymbolic {
			symbolicReasons[exp.SymbolicReason]++
			symbolicDetails = append(symbolicDetails, SymbolicDetail{
				Resource:  exp.Resource,
				Component: exp.CostUnit,
				Reason:    exp.SymbolicReason,
			})
		}
	}
	
	for resource, components := range grouped {
		isSymbolic := false
		for _, c := range components {
			if c.IsSymbolic {
				isSymbolic = true
				break
			}
		}
		
		response.ResourceExplanations = append(response.ResourceExplanations, ResourceExplanation{
			Address:    resource,
			IsSymbolic: isSymbolic,
			Components: components,
		})
	}
	
	if len(symbolicDetails) > 0 {
		response.SymbolicSummary = &SymbolicSummary{
			Count:   len(symbolicDetails),
			Reasons: symbolicReasons,
			Details: symbolicDetails,
		}
	}
	
	return response
}
