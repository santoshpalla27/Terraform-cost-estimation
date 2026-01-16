// Package pricing - Pricing gate enforcement
// No pricing may occur before provider finalization.
// This is enforced with panic, not error return.
package pricing

import (
	"fmt"
	"sync"
)

// PricingGate enforces that pricing only occurs after provider finalization
type PricingGate struct {
	mu              sync.RWMutex
	frozen          bool
	frozenProviders map[string]*FrozenProvider
}

// FrozenProvider is an immutable provider configuration
type FrozenProvider struct {
	Provider  string
	Alias     string
	Region    string
	Account   string
	frozen    bool
}

// NewPricingGate creates a new gate
func NewPricingGate() *PricingGate {
	return &PricingGate{
		frozenProviders: make(map[string]*FrozenProvider),
	}
}

// FreezeProvider freezes a provider configuration
func (g *PricingGate) FreezeProvider(provider, alias, region, account string) *FrozenProvider {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.frozen {
		panic("INVARIANT VIOLATED: cannot add providers after gate is frozen")
	}

	key := provider + ":" + alias
	fp := &FrozenProvider{
		Provider: provider,
		Alias:    alias,
		Region:   region,
		Account:  account,
		frozen:   true,
	}
	g.frozenProviders[key] = fp
	return fp
}

// Freeze freezes the gate - no more providers can be added
func (g *PricingGate) Freeze() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.frozen = true
}

// IsFrozen returns whether the gate is frozen
func (g *PricingGate) IsFrozen() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.frozen
}

// MustGetProvider returns a provider or panics
func (g *PricingGate) MustGetProvider(provider, alias string) *FrozenProvider {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.frozen {
		panic("INVARIANT VIOLATED: pricing before provider finalization")
	}

	key := provider + ":" + alias
	fp, ok := g.frozenProviders[key]
	if !ok {
		panic(fmt.Sprintf("INVARIANT VIOLATED: provider %s not frozen", key))
	}
	return fp
}

// AssertCanPrice panics if pricing is not allowed
func (g *PricingGate) AssertCanPrice() {
	if !g.IsFrozen() {
		panic("INVARIANT VIOLATED: pricing before provider finalization")
	}
}

// BuildRateKey builds a rate key with mandatory alias
func (fp *FrozenProvider) BuildRateKey(resourceType, component string) *AliasAwareRateKey {
	if !fp.frozen {
		panic("INVARIANT VIOLATED: building rate key from unfrozen provider")
	}
	return NewAliasAwareRateKey(fp.Provider, fp.Alias, fp.Region, resourceType, component)
}

// BuildRateKeyWithAccount builds a rate key with account
func (fp *FrozenProvider) BuildRateKeyWithAccount(resourceType, component string) *AliasAwareRateKey {
	if !fp.frozen {
		panic("INVARIANT VIOLATED: building rate key from unfrozen provider")
	}
	key := NewAliasAwareRateKey(fp.Provider, fp.Alias, fp.Region, resourceType, component)
	key.WithAccountID(fp.Account)
	return key
}

// String returns provider identity
func (fp *FrozenProvider) String() string {
	if fp.Alias != "" {
		return fp.Provider + "." + fp.Alias
	}
	return fp.Provider
}
