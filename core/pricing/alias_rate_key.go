// Package pricing - Alias-aware rate key
// Alias MUST be part of rate key - no exceptions
package pricing

import (
	"fmt"
)

// AliasAwareRateKey is a rate key that ALWAYS includes provider alias
type AliasAwareRateKey struct {
	Provider     string
	Alias        string  // REQUIRED - empty means default
	Region       string
	AccountID    string  // Optional but recommended
	ResourceType string
	Component    string
	SKU          string

	// Computed key
	key string
}

// NewAliasAwareRateKey creates a rate key with required alias
func NewAliasAwareRateKey(provider, alias, region, resourceType, component string) *AliasAwareRateKey {
	k := &AliasAwareRateKey{
		Provider:     provider,
		Alias:        alias,
		Region:       region,
		ResourceType: resourceType,
		Component:    component,
	}
	k.computeKey()
	return k
}

// WithAccountID adds account ID to the key
func (k *AliasAwareRateKey) WithAccountID(accountID string) *AliasAwareRateKey {
	k.AccountID = accountID
	k.computeKey()
	return k
}

// WithSKU adds SKU to the key
func (k *AliasAwareRateKey) WithSKU(sku string) *AliasAwareRateKey {
	k.SKU = sku
	k.computeKey()
	return k
}

func (k *AliasAwareRateKey) computeKey() {
	// Format: provider:alias:region:account:type:component:sku
	// Alias is ALWAYS included (empty = default)
	aliasStr := k.Alias
	if aliasStr == "" {
		aliasStr = "_default_"
	}
	
	accountStr := k.AccountID
	if accountStr == "" {
		accountStr = "_"
	}

	skuStr := k.SKU
	if skuStr == "" {
		skuStr = "_"
	}

	k.key = fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
		k.Provider, aliasStr, k.Region, accountStr,
		k.ResourceType, k.Component, skuStr,
	)
}

// Key returns the computed key string
func (k *AliasAwareRateKey) Key() string {
	return k.key
}

// String returns the key
func (k *AliasAwareRateKey) String() string {
	return k.key
}

// Matches checks if this key matches another
func (k *AliasAwareRateKey) Matches(other *AliasAwareRateKey) bool {
	return k.key == other.key
}

// MatchesProvider checks if provider/alias/region match
func (k *AliasAwareRateKey) MatchesProvider(provider, alias, region string) bool {
	return k.Provider == provider && k.Alias == alias && k.Region == region
}

// RateKeyBuilder builds alias-aware rate keys
type RateKeyBuilder struct {
	provider  string
	alias     string
	region    string
	accountID string
}

// NewRateKeyBuilder creates a builder
func NewRateKeyBuilder(provider, alias, region string) *RateKeyBuilder {
	return &RateKeyBuilder{
		provider: provider,
		alias:    alias,
		region:   region,
	}
}

// WithAccount sets account ID
func (b *RateKeyBuilder) WithAccount(accountID string) *RateKeyBuilder {
	b.accountID = accountID
	return b
}

// Build creates a rate key for a resource
func (b *RateKeyBuilder) Build(resourceType, component string) *AliasAwareRateKey {
	k := NewAliasAwareRateKey(b.provider, b.alias, b.region, resourceType, component)
	if b.accountID != "" {
		k.WithAccountID(b.accountID)
	}
	return k
}

// BuildWithSKU creates a rate key with SKU
func (b *RateKeyBuilder) BuildWithSKU(resourceType, component, sku string) *AliasAwareRateKey {
	k := NewAliasAwareRateKey(b.provider, b.alias, b.region, resourceType, component)
	if b.accountID != "" {
		k.WithAccountID(b.accountID)
	}
	k.WithSKU(sku)
	return k
}

// RateKeyValidator ensures rate keys include alias
type RateKeyValidator struct {
	errors []RateKeyError
}

// RateKeyError is an error in rate key construction
type RateKeyError struct {
	Context string
	Message string
}

// NewRateKeyValidator creates a validator
func NewRateKeyValidator() *RateKeyValidator {
	return &RateKeyValidator{
		errors: []RateKeyError{},
	}
}

// ValidateKey ensures a key is properly formed
func (v *RateKeyValidator) ValidateKey(key *AliasAwareRateKey, context string) bool {
	if key.Provider == "" {
		v.errors = append(v.errors, RateKeyError{
			Context: context,
			Message: "provider is required",
		})
		return false
	}
	if key.Region == "" {
		v.errors = append(v.errors, RateKeyError{
			Context: context,
			Message: "region is required",
		})
		return false
	}
	if key.ResourceType == "" {
		v.errors = append(v.errors, RateKeyError{
			Context: context,
			Message: "resource type is required",
		})
		return false
	}
	return true
}

// GetErrors returns all errors
func (v *RateKeyValidator) GetErrors() []RateKeyError {
	return v.errors
}

// AliasAwareRateResolver resolves rates with alias awareness
type AliasAwareRateResolver struct {
	validator *RateKeyValidator
}

// NewAliasAwareRateResolver creates a resolver
func NewAliasAwareRateResolver() *AliasAwareRateResolver {
	return &AliasAwareRateResolver{
		validator: NewRateKeyValidator(),
	}
}

// ResolveRate resolves a rate, ensuring alias is included
func (r *AliasAwareRateResolver) ResolveRate(snapshot *PricingSnapshot, key *AliasAwareRateKey) (*RateEntry, error) {
	if !r.validator.ValidateKey(key, key.ResourceType) {
		return nil, fmt.Errorf("invalid rate key for %s", key.ResourceType)
	}

	// Lookup with full alias-aware key
	rate, ok := snapshot.LookupRateByKey(key.Key())
	if !ok {
		// Try fallback to default alias
		if key.Alias != "" {
			fallbackKey := NewAliasAwareRateKey(key.Provider, "", key.Region, key.ResourceType, key.Component)
			rate, ok = snapshot.LookupRateByKey(fallbackKey.Key())
			if ok {
				return rate, nil
			}
		}
		return nil, &RateNotFoundError{Key: key.Key()}
	}

	return rate, nil
}

// RateNotFoundError indicates a rate was not found
type RateNotFoundError struct {
	Key string
}

func (e *RateNotFoundError) Error() string {
	return "rate not found: " + e.Key
}

// LookupRateByKey looks up a rate by key string
func (s *PricingSnapshot) LookupRateByKey(key string) (*RateEntry, bool) {
	for i := range s.rates {
		if s.rates[i].Key.String() == key {
			return &s.rates[i], true
		}
	}
	return nil, false
}
