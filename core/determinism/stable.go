// Package determinism provides primitives for guaranteeing deterministic execution.
// All code must use these primitives instead of Go built-ins for maps, IDs, etc.
package determinism

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"github.com/shopspring/decimal"
)

// StableMap is a map that guarantees iteration order (sorted by key).
// Use this instead of map[K]V for all cases where iteration matters.
type StableMap[K comparable, V any] struct {
	mu      sync.RWMutex
	keys    []K
	values  map[K]V
	keyFunc func(K) string // For custom ordering
}

// NewStableMap creates a new StableMap
func NewStableMap[K comparable, V any]() *StableMap[K, V] {
	return &StableMap[K, V]{
		values: make(map[K]V),
	}
}

// NewStableMapWithKeyFunc creates a StableMap with custom key ordering
func NewStableMapWithKeyFunc[K comparable, V any](keyFunc func(K) string) *StableMap[K, V] {
	return &StableMap[K, V]{
		values:  make(map[K]V),
		keyFunc: keyFunc,
	}
}

// Set adds or updates a key-value pair
func (m *StableMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
		m.sortKeys()
	}
	m.values[key] = value
}

// Get retrieves a value by key
func (m *StableMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.values[key]
	return val, ok
}

// Delete removes a key
func (m *StableMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.values, key)
	// Remove from keys slice
	for i, k := range m.keys {
		if any(k) == any(key) {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// Range iterates in stable sorted order
func (m *StableMap[K, V]) Range(fn func(K, V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, k := range m.keys {
		if !fn(k, m.values[k]) {
			break
		}
	}
}

// Keys returns all keys in sorted order
func (m *StableMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]K, len(m.keys))
	copy(result, m.keys)
	return result
}

// Len returns the number of entries
func (m *StableMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.values)
}

func (m *StableMap[K, V]) sortKeys() {
	sort.Slice(m.keys, func(i, j int) bool {
		if m.keyFunc != nil {
			return m.keyFunc(m.keys[i]) < m.keyFunc(m.keys[j])
		}
		return fmt.Sprint(m.keys[i]) < fmt.Sprint(m.keys[j])
	})
}

// StableID is a hash-based unique identifier that's deterministic
type StableID string

// IDGenerator generates stable, deterministic IDs
type IDGenerator struct {
	namespace string
}

// NewIDGenerator creates an ID generator with a namespace
func NewIDGenerator(namespace string) *IDGenerator {
	return &IDGenerator{namespace: namespace}
}

// Generate creates a stable ID from inputs
func (g *IDGenerator) Generate(parts ...string) StableID {
	h := sha256.New()
	h.Write([]byte(g.namespace))
	h.Write([]byte{0}) // Separator
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0}) // Separator
	}
	return StableID(hex.EncodeToString(h.Sum(nil))[:16])
}

// ContentHash is a SHA-256 hash for content integrity
type ContentHash [32]byte

// ComputeHash computes a content hash from bytes
func ComputeHash(data []byte) ContentHash {
	return sha256.Sum256(data)
}

// Hex returns the hash as a hex string
func (h ContentHash) Hex() string {
	return hex.EncodeToString(h[:])
}

// String implements Stringer
func (h ContentHash) String() string {
	return h.Hex()[:16] + "..."
}

// Money represents a monetary amount with full precision.
// NEVER use float64 for money calculations.
type Money struct {
	amount   decimal.Decimal
	currency string
}

// NewMoney creates a Money from a decimal string
func NewMoney(amount string, currency string) (Money, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: d, currency: currency}, nil
}

// NewMoneyFromFloat creates Money from float64 (use sparingly)
func NewMoneyFromFloat(amount float64, currency string) Money {
	return Money{amount: decimal.NewFromFloat(amount), currency: currency}
}

// NewMoneyFromDecimal creates Money from decimal
func NewMoneyFromDecimal(amount decimal.Decimal, currency string) Money {
	return Money{amount: amount, currency: currency}
}

// Zero creates zero money
func Zero(currency string) Money {
	return Money{amount: decimal.Zero, currency: currency}
}

// Amount returns the decimal amount
func (m Money) Amount() decimal.Decimal {
	return m.amount
}

// Currency returns the currency code
func (m Money) Currency() string {
	return m.currency
}

// Add adds two monetary amounts
func (m Money) Add(other Money) Money {
	if m.currency != other.currency {
		panic(fmt.Sprintf("cannot add %s and %s", m.currency, other.currency))
	}
	return Money{amount: m.amount.Add(other.amount), currency: m.currency}
}

// Sub subtracts monetary amounts
func (m Money) Sub(other Money) Money {
	if m.currency != other.currency {
		panic(fmt.Sprintf("cannot subtract %s and %s", m.currency, other.currency))
	}
	return Money{amount: m.amount.Sub(other.amount), currency: m.currency}
}

// Mul multiplies by a scalar
func (m Money) Mul(factor decimal.Decimal) Money {
	return Money{amount: m.amount.Mul(factor), currency: m.currency}
}

// MulFloat multiplies by a float64 scalar
func (m Money) MulFloat(factor float64) Money {
	return Money{amount: m.amount.Mul(decimal.NewFromFloat(factor)), currency: m.currency}
}

// Div divides by a scalar
func (m Money) Div(divisor decimal.Decimal) Money {
	return Money{amount: m.amount.Div(divisor), currency: m.currency}
}

// IsZero returns true if amount is zero
func (m Money) IsZero() bool {
	return m.amount.IsZero()
}

// IsNegative returns true if amount is negative
func (m Money) IsNegative() bool {
	return m.amount.IsNegative()
}

// Cmp compares two monetary amounts
func (m Money) Cmp(other Money) int {
	if m.currency != other.currency {
		panic(fmt.Sprintf("cannot compare %s and %s", m.currency, other.currency))
	}
	return m.amount.Cmp(other.amount)
}

// String returns formatted money (2 decimal places)
func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.amount.StringFixed(2), m.currency)
}

// StringRaw returns the raw decimal string (full precision)
func (m Money) StringRaw() string {
	return m.amount.String()
}

// Float64 returns float64 (only for display, never for calculation)
func (m Money) Float64() float64 {
	f, _ := m.amount.Float64()
	return f
}

// SortSlice sorts a slice in a stable, deterministic manner
func SortSlice[T any](slice []T, less func(a, b T) bool) {
	sort.SliceStable(slice, func(i, j int) bool {
		return less(slice[i], slice[j])
	})
}

// SortStrings sorts strings in place
func SortStrings(s []string) {
	sort.Strings(s)
}

// SortedMap returns a sorted copy of map keys
func SortedKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j])
	})
	return keys
}

// RangeMapSorted iterates over a map in sorted key order
func RangeMapSorted[K comparable, V any](m map[K]V, fn func(K, V) bool) {
	for _, k := range SortedKeys(m) {
		if !fn(k, m[k]) {
			break
		}
	}
}
