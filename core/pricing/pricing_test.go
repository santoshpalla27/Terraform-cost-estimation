// Package pricing - Pricing invariant tests
// These tests PROVE the invariants are real by intentionally violating them.
package pricing

import (
	"testing"
)

// TestPricingBeforeFinalizationPanics proves pricing cannot occur before finalization
func TestPricingBeforeFinalizationPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic when pricing before finalization, but no panic occurred")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected string panic, got %T: %v", r, r)
		}
		if len(msg) < 10 {
			t.Fatalf("Panic message too short: %s", msg)
		}
		t.Logf("Correctly panicked: %s", msg)
	}()

	// Create gate but DO NOT freeze it
	gate := NewPricingGate()
	gate.FreezeProvider("aws", "prod", "us-east-1", "123456789")
	// gate.Freeze() - intentionally NOT called

	// This MUST panic
	gate.AssertCanPrice()
}

// TestRateKeyWithoutAliasPanics proves alias is mandatory
func TestRateKeyWithoutAliasPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for RateKey without finalized provider, but no panic occurred")
		}
		t.Logf("Correctly panicked: %v", r)
	}()

	// Create unfinalized provider
	unfinalizedProvider := &unfinalizedProviderMock{}

	// This MUST panic
	_ = NewRateKeyStrict(unfinalizedProvider, "ec2", "us-east-1", nil)
}

// unfinalizedProviderMock is a mock that is NOT finalized
type unfinalizedProviderMock struct{}

func (m *unfinalizedProviderMock) Provider() string  { return "aws" }
func (m *unfinalizedProviderMock) Alias() string     { return "" }
func (m *unfinalizedProviderMock) IsFinalized() bool { return false }

// TestPricingGateFreezeWorks proves freezing works correctly
func TestPricingGateFreezeWorks(t *testing.T) {
	gate := NewPricingGate()
	gate.FreezeProvider("aws", "prod", "us-east-1", "123456789")
	gate.Freeze()

	// This should NOT panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Unexpected panic after proper freezing: %v", r)
		}
	}()

	gate.AssertCanPrice()
	provider := gate.MustGetProvider("aws", "prod")
	if provider.Provider != "aws" {
		t.Errorf("Expected provider 'aws', got '%s'", provider.Provider)
	}
	if provider.Alias != "prod" {
		t.Errorf("Expected alias 'prod', got '%s'", provider.Alias)
	}
}

// TestRateKeyIncludesAlias proves alias is embedded in rate key
func TestRateKeyIncludesAlias(t *testing.T) {
	provider := NewFinalizedProvider("aws", "prod", "us-east-1", "123456789")
	rateKey := NewRateKeyStrict(provider, "ec2", "us-east-1", nil)

	key := rateKey.Key()
	if key == "" {
		t.Fatal("RateKey.Key() returned empty string")
	}

	// Key should include alias
	providerKey := rateKey.ProviderFullKey()
	if providerKey != "aws.prod" {
		t.Errorf("Expected ProviderFullKey 'aws.prod', got '%s'", providerKey)
	}

	t.Logf("RateKey: %s, ProviderFullKey: %s", key, providerKey)
}

// TestScopedPricingSnapshot proves snapshots are alias-scoped
func TestScopedPricingSnapshot(t *testing.T) {
	// Create two snapshots for different aliases
	snapshotProd := NewScopedPricingSnapshot("aws.prod", "us-east-1")
	snapshotDev := NewScopedPricingSnapshot("aws.dev", "us-east-1")

	// Add same rate key to both
	snapshotProd.AddRate("aws.prod:us-east-1:ec2", ScopedRate{
		Key:      "aws.prod:us-east-1:ec2",
		Price:    "0.10",
		Currency: "USD",
		Unit:     "hour",
	})
	snapshotDev.AddRate("aws.dev:us-east-1:ec2", ScopedRate{
		Key:      "aws.dev:us-east-1:ec2",
		Price:    "0.05",
		Currency: "USD",
		Unit:     "hour",
	})

	// Verify isolation
	providerProd := NewFinalizedProvider("aws", "prod", "us-east-1", "123456789")
	providerDev := NewFinalizedProvider("aws", "dev", "us-east-1", "987654321")

	rateKeyProd := NewRateKeyStrict(providerProd, "ec2", "us-east-1", nil)
	rateKeyDev := NewRateKeyStrict(providerDev, "ec2", "us-east-1", nil)

	// Prod snapshot should not return dev rate
	_, found := snapshotProd.GetRate(rateKeyDev)
	if found {
		t.Error("Prod snapshot should not return dev rate - snapshots not properly isolated")
	}

	// Dev snapshot should not return prod rate
	_, found = snapshotDev.GetRate(rateKeyProd)
	if found {
		t.Error("Dev snapshot should not return prod rate - snapshots not properly isolated")
	}

	t.Log("Pricing snapshots are correctly alias-scoped")
}
