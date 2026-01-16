// Package mapper - Helper functions for building usage and cost units
package mapper

// NewUsageVector creates a concrete usage vector
func NewUsageVector(metric string, value float64, confidence float64) UsageVector {
	return UsageVector{
		Metric:     metric,
		Value:      &value,
		Confidence: confidence,
	}
}

// SymbolicUsage creates a symbolic usage vector
func SymbolicUsage(metric string, reason string) UsageVector {
	return UsageVector{
		Metric:         metric,
		IsSymbolic:     true,
		SymbolicReason: reason,
		Confidence:     0,
	}
}

// NewCostUnit creates a concrete cost unit
func NewCostUnit(name, measure string, quantity float64, rateKey RateKey, confidence float64) CostUnit {
	return CostUnit{
		Name:       name,
		Measure:    measure,
		Quantity:   &quantity,
		RateKey:    rateKey,
		Confidence: confidence,
	}
}

// SymbolicCost creates a symbolic cost unit
func SymbolicCost(name, reason string) CostUnit {
	return CostUnit{
		Name:           name,
		IsSymbolic:     true,
		SymbolicReason: reason,
		Confidence:     0,
	}
}

// Common metrics
const (
	MetricMonthlyHours    = "monthly_hours"
	MetricMonthlyRequests = "monthly_requests"
	MetricStorageGB       = "storage_gb"
	MetricDataTransferGB  = "data_transfer_gb"
	MetricIOPS            = "iops"
	MetricThroughputMBps  = "throughput_mbps"
)
