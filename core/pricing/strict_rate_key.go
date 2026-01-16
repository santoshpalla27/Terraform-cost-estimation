// Package pricing - RateKey constructor enforcement
// RateKeys can ONLY be created via constructor - no struct literals allowed.
// This ensures provider alias is always present.
package pricing

import (
	"fmt"
	"strings"
)

// RateKeyStrict is a rate key that can ONLY be created via constructor
type RateKeyStrict struct {
	provider  string
	alias     string
	region    string
	service   string
	sku       string
	attrs     map[string]string
	validated bool
}

// NewRateKeyStrict creates a rate key with REQUIRED provider identity
// Panics if provider is not finalized
func NewRateKeyStrict(provider ProviderIdentity, service, region string, attrs map[string]string) *RateKeyStrict {
	if !provider.IsFinalized() {
		panic("INVARIANT VIOLATED: pricing requested before provider finalization")
	}

	key := &RateKeyStrict{
		provider:  provider.Provider(),
		alias:     provider.Alias(),
		region:    region,
		service:   service,
		attrs:     attrs,
		validated: true,
	}

	return key
}

// ProviderIdentity represents a finalized provider
type ProviderIdentity interface {
	Provider() string
	Alias() string
	IsFinalized() bool
}

// FinalizedProvider is a provider that has been frozen
type FinalizedProvider struct {
	provider  string
	alias     string
	region    string
	account   string
	finalized bool
}

// NewFinalizedProvider creates a finalized provider
func NewFinalizedProvider(provider, alias, region, account string) *FinalizedProvider {
	return &FinalizedProvider{
		provider:  provider,
		alias:     alias,
		region:    region,
		account:   account,
		finalized: true,
	}
}

// Provider returns the provider name
func (p *FinalizedProvider) Provider() string { return p.provider }

// Alias returns the alias
func (p *FinalizedProvider) Alias() string { return p.alias }

// Region returns the region
func (p *FinalizedProvider) Region() string { return p.region }

// Account returns the account
func (p *FinalizedProvider) Account() string { return p.account }

// IsFinalized returns true if finalized
func (p *FinalizedProvider) IsFinalized() bool { return p.finalized }

// FullKey returns the full provider key (provider.alias)
func (p *FinalizedProvider) FullKey() string {
	if p.alias == "" || p.alias == "default" {
		return p.provider
	}
	return p.provider + "." + p.alias
}

// Key returns the lookup key for this rate
func (k *RateKeyStrict) Key() string {
	if !k.validated {
		panic("INVARIANT VIOLATED: using unvalidated RateKey")
	}

	parts := []string{k.provider}
	if k.alias != "" && k.alias != "default" {
		parts = append(parts, k.alias)
	}
	parts = append(parts, k.region, k.service)
	if k.sku != "" {
		parts = append(parts, k.sku)
	}

	return strings.Join(parts, ":")
}

// WithSKU adds SKU to the key
func (k *RateKeyStrict) WithSKU(sku string) *RateKeyStrict {
	k.sku = sku
	return k
}

// WithAttr adds an attribute
func (k *RateKeyStrict) WithAttr(key, value string) *RateKeyStrict {
	if k.attrs == nil {
		k.attrs = make(map[string]string)
	}
	k.attrs[key] = value
	return k
}

// String returns string representation
func (k *RateKeyStrict) String() string {
	return k.Key()
}

// ProviderFullKey returns provider.alias
func (k *RateKeyStrict) ProviderFullKey() string {
	if k.alias == "" || k.alias == "default" {
		return k.provider
	}
	return k.provider + "." + k.alias
}

// ValidateProviderFinalization validates that provider is finalized
func ValidateProviderFinalization(provider ProviderIdentity) error {
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}
	if !provider.IsFinalized() {
		return fmt.Errorf("provider %s is not finalized", provider.Provider())
	}
	return nil
}

// MustBeFinalized panics if provider is not finalized
func MustBeFinalized(provider ProviderIdentity) {
	if err := ValidateProviderFinalization(provider); err != nil {
		panic("INVARIANT VIOLATED: " + err.Error())
	}
}

// PricingSnapshot must be scoped by provider + alias + region
type ScopedPricingSnapshot struct {
	// Scope
	ProviderKey string // provider.alias
	Region      string

	// Rates
	rates map[string]ScopedRate

	// Metadata
	Version     string
	EffectiveAt string
	ContentHash string
}

// ScopedRate is a rate scoped to provider + alias + region
type ScopedRate struct {
	Key      string
	Price    string // decimal
	Currency string
	Unit     string
}

// NewScopedPricingSnapshot creates a scoped snapshot
func NewScopedPricingSnapshot(providerKey, region string) *ScopedPricingSnapshot {
	return &ScopedPricingSnapshot{
		ProviderKey: providerKey,
		Region:      region,
		rates:       make(map[string]ScopedRate),
	}
}

// AddRate adds a rate
func (s *ScopedPricingSnapshot) AddRate(key string, rate ScopedRate) {
	s.rates[key] = rate
}

// GetRate returns a rate
func (s *ScopedPricingSnapshot) GetRate(key *RateKeyStrict) (*ScopedRate, bool) {
	// Validate scope
	if key.ProviderFullKey() != s.ProviderKey {
		return nil, false
	}
	if key.region != s.Region {
		return nil, false
	}

	rate, ok := s.rates[key.Key()]
	if !ok {
		return nil, false
	}
	return &rate, true
}
