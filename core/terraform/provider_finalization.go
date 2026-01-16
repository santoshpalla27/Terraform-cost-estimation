// Package terraform - Provider alias finalization
// Provider context MUST be frozen before pricing resolution.
// Alias + region + account is a first-class dimension.
package terraform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// FrozenProviderContext is an immutable provider binding
type FrozenProviderContext struct {
	// Canonical identity
	ProviderKey string // e.g., "aws.us_east"

	// Provider details
	Type   string
	Alias  string
	Region string

	// Account context
	AccountID string
	AssumeRole string

	// Content hash (immutable)
	ContentHash string

	// Is this frozen?
	frozen bool
}

// Freeze creates an immutable copy
func (p *ProviderContext) Freeze() *FrozenProviderContext {
	frozen := &FrozenProviderContext{
		ProviderKey: p.ProviderKey(),
		Type:        p.ProviderType,
		Alias:       p.Alias,
		Region:      p.Region,
		frozen:      true,
	}

	// Extract account from config if present
	if p.Config != nil {
		if accountID, ok := p.Config["account_id"].(string); ok {
			frozen.AccountID = accountID
		}
		if roleArn, ok := p.Config["assume_role"].(string); ok {
			frozen.AssumeRole = roleArn
		}
	}

	// Compute content hash
	frozen.ContentHash = frozen.computeHash()

	return frozen
}

func (f *FrozenProviderContext) computeHash() string {
	h := sha256.New()
	h.Write([]byte(f.Type))
	h.Write([]byte(f.Alias))
	h.Write([]byte(f.Region))
	h.Write([]byte(f.AccountID))
	h.Write([]byte(f.AssumeRole))
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// RateKey returns the key for pricing lookup
func (f *FrozenProviderContext) RateKey(resourceType, sku string) string {
	// Format: provider:region:type:sku
	return fmt.Sprintf("%s:%s:%s:%s", f.Type, f.Region, resourceType, sku)
}

// PricingDimension returns all dimensions for pricing
func (f *FrozenProviderContext) PricingDimension() PricingDimension {
	return PricingDimension{
		Provider:  f.Type,
		Region:    f.Region,
		AccountID: f.AccountID,
		Alias:     f.Alias,
	}
}

// PricingDimension contains all provider dimensions for pricing
type PricingDimension struct {
	Provider  string
	Region    string
	AccountID string
	Alias     string
}

// Key returns a unique key for this dimension
func (d PricingDimension) Key() string {
	if d.AccountID != "" {
		return fmt.Sprintf("%s:%s:%s", d.Provider, d.Region, d.AccountID)
	}
	return fmt.Sprintf("%s:%s", d.Provider, d.Region)
}

// ProviderFinalizer ensures providers are frozen before use
type ProviderFinalizer struct {
	// All frozen providers
	frozen map[string]*FrozenProviderContext

	// Finalization order
	order []string

	// Is finalization complete?
	finalized bool
}

// NewProviderFinalizer creates a finalizer
func NewProviderFinalizer() *ProviderFinalizer {
	return &ProviderFinalizer{
		frozen:    make(map[string]*FrozenProviderContext),
		order:     []string{},
		finalized: false,
	}
}

// Freeze freezes a provider context
func (pf *ProviderFinalizer) Freeze(ctx *ProviderContext) (*FrozenProviderContext, error) {
	if pf.finalized {
		return nil, fmt.Errorf("provider finalization already complete")
	}

	frozen := ctx.Freeze()
	pf.frozen[frozen.ProviderKey] = frozen
	pf.order = append(pf.order, frozen.ProviderKey)

	return frozen, nil
}

// Finalize marks finalization complete - no more providers can be added
func (pf *ProviderFinalizer) Finalize() {
	pf.finalized = true
}

// Get returns a frozen provider
func (pf *ProviderFinalizer) Get(key string) (*FrozenProviderContext, bool) {
	frozen, ok := pf.frozen[key]
	return frozen, ok
}

// MustGet returns a frozen provider or panics
func (pf *ProviderFinalizer) MustGet(key string) *FrozenProviderContext {
	frozen, ok := pf.frozen[key]
	if !ok {
		panic(fmt.Sprintf("provider %s not frozen", key))
	}
	return frozen
}

// All returns all frozen providers
func (pf *ProviderFinalizer) All() []*FrozenProviderContext {
	result := make([]*FrozenProviderContext, 0, len(pf.frozen))
	for _, key := range pf.order {
		result = append(result, pf.frozen[key])
	}
	return result
}

// IsFinalized returns true if finalization is complete
func (pf *ProviderFinalizer) IsFinalized() bool {
	return pf.finalized
}

// InstanceProviderBinding binds an instance to its frozen provider
type InstanceProviderBinding struct {
	InstanceAddress string
	InstanceKey     interface{}
	Provider        *FrozenProviderContext
	BoundAt         string // When binding was established
}

// BindingRegistry tracks all instance-provider bindings
type BindingRegistry struct {
	bindings map[string]*InstanceProviderBinding
}

// NewBindingRegistry creates a registry
func NewBindingRegistry() *BindingRegistry {
	return &BindingRegistry{
		bindings: make(map[string]*InstanceProviderBinding),
	}
}

// Bind binds an instance to a provider
func (r *BindingRegistry) Bind(address string, key interface{}, provider *FrozenProviderContext) {
	r.bindings[address] = &InstanceProviderBinding{
		InstanceAddress: address,
		InstanceKey:     key,
		Provider:        provider,
		BoundAt:         "expansion",
	}
}

// Get returns the binding for an instance
func (r *BindingRegistry) Get(address string) (*InstanceProviderBinding, bool) {
	binding, ok := r.bindings[address]
	return binding, ok
}

// MustGet returns the binding or panics
func (r *BindingRegistry) MustGet(address string) *InstanceProviderBinding {
	binding, ok := r.bindings[address]
	if !ok {
		panic(fmt.Sprintf("no provider binding for %s", address))
	}
	return binding
}

// EnsureBound verifies an instance is bound before pricing
func (r *BindingRegistry) EnsureBound(address string) error {
	if _, ok := r.bindings[address]; !ok {
		return &UnboundInstanceError{Address: address}
	}
	return nil
}

// UnboundInstanceError indicates an instance has no provider binding
type UnboundInstanceError struct {
	Address string
}

func (e *UnboundInstanceError) Error() string {
	return fmt.Sprintf("instance %s has no provider binding - cannot price", e.Address)
}

// ProviderPricingGate ensures pricing only happens after provider finalization
type ProviderPricingGate struct {
	finalizer *ProviderFinalizer
	registry  *BindingRegistry
}

// NewProviderPricingGate creates a gate
func NewProviderPricingGate(finalizer *ProviderFinalizer, registry *BindingRegistry) *ProviderPricingGate {
	return &ProviderPricingGate{
		finalizer: finalizer,
		registry:  registry,
	}
}

// CanPrice checks if pricing is allowed for an instance
func (g *ProviderPricingGate) CanPrice(address string) error {
	// Check finalization
	if !g.finalizer.IsFinalized() {
		return fmt.Errorf("provider finalization not complete - cannot price %s", address)
	}

	// Check binding
	return g.registry.EnsureBound(address)
}

// GetPricingDimension returns the pricing dimension for an instance
func (g *ProviderPricingGate) GetPricingDimension(address string) (*PricingDimension, error) {
	if err := g.CanPrice(address); err != nil {
		return nil, err
	}

	binding := g.registry.MustGet(address)
	dim := binding.Provider.PricingDimension()
	return &dim, nil
}
