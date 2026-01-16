// Package catalog - Catalog validation
// Ensures catalog integrity and enforces invariants.
package catalog

import (
	"fmt"
)

// ValidationRule is a catalog validation rule
type ValidationRule func(*ResourceEntry) error

// DefaultValidationRules returns the standard validation rules
func DefaultValidationRules() []ValidationRule {
	return []ValidationRule{
		validateTierBehaviorConsistency,
		validateUsageRequirement,
		validateMapperRequirement,
	}
}

// Validate checks a catalog against validation rules
func (c *Catalog) Validate(rules []ValidationRule) []error {
	var errors []error
	
	for _, entry := range c.entries {
		for _, rule := range rules {
			if err := rule(entry); err != nil {
				errors = append(errors, fmt.Errorf("%s:%s: %w", entry.Cloud, entry.ResourceType, err))
			}
		}
	}
	
	return errors
}

// validateTierBehaviorConsistency ensures tier and behavior are consistent
func validateTierBehaviorConsistency(e *ResourceEntry) error {
	switch e.Tier {
	case Tier1Numeric:
		if e.Behavior == CostIndirect {
			return fmt.Errorf("Tier1 cannot have CostIndirect behavior")
		}
	case Tier3Indirect:
		if e.Behavior != CostIndirect {
			return fmt.Errorf("Tier3 must have CostIndirect behavior")
		}
	}
	return nil
}

// validateUsageRequirement ensures usage-based resources are flagged
func validateUsageRequirement(e *ResourceEntry) error {
	if e.Behavior == CostUsageBased && !e.RequiresUsage {
		return fmt.Errorf("CostUsageBased behavior requires RequiresUsage=true")
	}
	return nil
}

// validateMapperRequirement ensures Tier1 with mapper has correct behavior
func validateMapperRequirement(e *ResourceEntry) error {
	if e.MapperExists && e.Tier == Tier3Indirect {
		return fmt.Errorf("Tier3 (indirect) resources should not have numeric mappers")
	}
	return nil
}

// MustValidate panics if validation fails
func (c *Catalog) MustValidate() {
	errors := c.Validate(DefaultValidationRules())
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Printf("Catalog validation error: %v\n", err)
		}
		panic(fmt.Sprintf("Catalog has %d validation errors", len(errors)))
	}
}

// GlobalCatalog is the default global catalog
var GlobalCatalog = NewCatalog()

// Init initializes the global catalog with all clouds
func Init() {
	RegisterAWS(GlobalCatalog)
	RegisterAzure(GlobalCatalog)
	RegisterGCP(GlobalCatalog)
	GlobalCatalog.MustValidate()
}
