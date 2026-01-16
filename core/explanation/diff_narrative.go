// Package explanation - Diff narratives
// Explains what changed and why costs differ
package explanation

import (
	"fmt"
	"strings"
)

// DiffNarrative explains a cost difference
type DiffNarrative struct {
	Resource     string        `json:"resource"`
	OldCost      float64       `json:"old_cost"`
	NewCost      float64       `json:"new_cost"`
	CostDelta    float64       `json:"cost_delta"`
	ChangeType   string        `json:"change_type"` // "create", "destroy", "update", "no_change"
	Changes      []ChangeItem  `json:"changes"`
	Narrative    string        `json:"narrative"`
}

// ChangeItem represents a single attribute change
type ChangeItem struct {
	Attribute string `json:"attribute"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
	Impact    string `json:"impact"` // "increase", "decrease", "neutral"
	CostImpact float64 `json:"cost_impact,omitempty"`
}

// NewDiffNarrative creates a diff narrative
func NewDiffNarrative(resource string, oldCost, newCost float64) *DiffNarrative {
	changeType := "update"
	if oldCost == 0 {
		changeType = "create"
	} else if newCost == 0 {
		changeType = "destroy"
	} else if oldCost == newCost {
		changeType = "no_change"
	}
	
	return &DiffNarrative{
		Resource:   resource,
		OldCost:    oldCost,
		NewCost:    newCost,
		CostDelta:  newCost - oldCost,
		ChangeType: changeType,
		Changes:    make([]ChangeItem, 0),
	}
}

// AddChange adds an attribute change
func (d *DiffNarrative) AddChange(attr, oldVal, newVal string, costImpact float64) *DiffNarrative {
	impact := "neutral"
	if costImpact > 0 {
		impact = "increase"
	} else if costImpact < 0 {
		impact = "decrease"
	}
	
	d.Changes = append(d.Changes, ChangeItem{
		Attribute:  attr,
		OldValue:   oldVal,
		NewValue:   newVal,
		Impact:     impact,
		CostImpact: costImpact,
	})
	return d
}

// Build generates the narrative text
func (d *DiffNarrative) Build() *DiffNarrative {
	var parts []string
	
	switch d.ChangeType {
	case "create":
		parts = append(parts, fmt.Sprintf("New resource %s will cost $%.2f/month", d.Resource, d.NewCost))
		
	case "destroy":
		parts = append(parts, fmt.Sprintf("Removing %s will save $%.2f/month", d.Resource, d.OldCost))
		
	case "no_change":
		parts = append(parts, fmt.Sprintf("%s cost unchanged at $%.2f/month", d.Resource, d.NewCost))
		
	case "update":
		if d.CostDelta > 0 {
			parts = append(parts, fmt.Sprintf("%s cost increased by $%.2f (from $%.2f to $%.2f)", 
				d.Resource, d.CostDelta, d.OldCost, d.NewCost))
		} else {
			parts = append(parts, fmt.Sprintf("%s cost decreased by $%.2f (from $%.2f to $%.2f)", 
				d.Resource, -d.CostDelta, d.OldCost, d.NewCost))
		}
	}
	
	if len(d.Changes) > 0 {
		parts = append(parts, "because:")
		for _, change := range d.Changes {
			if change.OldValue == "" {
				parts = append(parts, fmt.Sprintf("  • %s set to %s", change.Attribute, change.NewValue))
			} else if change.NewValue == "" {
				parts = append(parts, fmt.Sprintf("  • %s removed (was %s)", change.Attribute, change.OldValue))
			} else {
				parts = append(parts, fmt.Sprintf("  • %s changed: %s → %s", change.Attribute, change.OldValue, change.NewValue))
			}
		}
	}
	
	d.Narrative = strings.Join(parts, "\n")
	return d
}

// ToMarkdown returns markdown formatted narrative
func (d *DiffNarrative) ToMarkdown() string {
	var sb strings.Builder
	
	// Header with emoji
	switch d.ChangeType {
	case "create":
		sb.WriteString("➕ ")
	case "destroy":
		sb.WriteString("➖ ")
	case "update":
		if d.CostDelta > 0 {
			sb.WriteString("📈 ")
		} else if d.CostDelta < 0 {
			sb.WriteString("📉 ")
		} else {
			sb.WriteString("🔄 ")
		}
	default:
		sb.WriteString("○ ")
	}
	
	sb.WriteString(fmt.Sprintf("**%s**\n\n", d.Resource))
	
	// Cost summary
	switch d.ChangeType {
	case "create":
		sb.WriteString(fmt.Sprintf("New resource: **+$%.2f/month**\n\n", d.NewCost))
	case "destroy":
		sb.WriteString(fmt.Sprintf("Removing: **-$%.2f/month**\n\n", d.OldCost))
	default:
		if d.CostDelta != 0 {
			sign := "+"
			if d.CostDelta < 0 {
				sign = ""
			}
			sb.WriteString(fmt.Sprintf("Cost change: **%s$%.2f/month** ($%.2f → $%.2f)\n\n", 
				sign, d.CostDelta, d.OldCost, d.NewCost))
		}
	}
	
	// Changes
	if len(d.Changes) > 0 {
		sb.WriteString("**Changes:**\n")
		for _, change := range d.Changes {
			icon := "•"
			if change.Impact == "increase" {
				icon = "🔺"
			} else if change.Impact == "decrease" {
				icon = "🔻"
			}
			
			if change.OldValue == "" {
				sb.WriteString(fmt.Sprintf("- %s `%s` = `%s`\n", icon, change.Attribute, change.NewValue))
			} else if change.NewValue == "" {
				sb.WriteString(fmt.Sprintf("- %s `%s` removed\n", icon, change.Attribute))
			} else {
				sb.WriteString(fmt.Sprintf("- %s `%s`: `%s` → `%s`\n", icon, change.Attribute, change.OldValue, change.NewValue))
			}
		}
	}
	
	return sb.String()
}

// ToPRComment returns a compact PR comment format
func (d *DiffNarrative) ToPRComment() string {
	switch d.ChangeType {
	case "create":
		return fmt.Sprintf("| `%s` | - | $%.2f | +$%.2f |", d.Resource, d.NewCost, d.NewCost)
	case "destroy":
		return fmt.Sprintf("| `%s` | $%.2f | - | -$%.2f |", d.Resource, d.OldCost, d.OldCost)
	case "update":
		sign := "+"
		if d.CostDelta < 0 {
			sign = ""
		}
		return fmt.Sprintf("| `%s` | $%.2f | $%.2f | %s$%.2f |", d.Resource, d.OldCost, d.NewCost, sign, d.CostDelta)
	default:
		return fmt.Sprintf("| `%s` | $%.2f | $%.2f | $0.00 |", d.Resource, d.OldCost, d.NewCost)
	}
}
