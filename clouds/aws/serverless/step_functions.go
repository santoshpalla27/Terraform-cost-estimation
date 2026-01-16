// Package serverless - AWS Step Functions mapper
package serverless

import (
	"terraform-cost/clouds"
)

// StepFunctionsMapper maps aws_sfn_state_machine to cost units
type StepFunctionsMapper struct{}

func NewStepFunctionsMapper() *StepFunctionsMapper { return &StepFunctionsMapper{} }

func (m *StepFunctionsMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *StepFunctionsMapper) ResourceType() string        { return "aws_sfn_state_machine" }

func (m *StepFunctionsMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage("state_machines", "unknown state machine count")}, nil
	}

	transitions := ctx.ResolveOrDefault("monthly_transitions", -1)
	if transitions < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage("transitions", "Step Functions transitions not provided"),
		}, nil
	}

	return []clouds.UsageVector{clouds.NewUsageVector("transitions", transitions, 0.5)}, nil
}

func (m *StepFunctionsMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		smType := asset.Attr("type")
		if smType == "" {
			smType = "STANDARD"
		}
		return []clouds.CostUnit{
			clouds.SymbolicCost("step_functions", "Step Functions ("+smType+") cost depends on state transitions"),
		}, nil
	}

	transitions, _ := usageVecs.Get("transitions")
	smType := asset.Attr("type")
	if smType == "" {
		smType = "STANDARD"
	}

	usageType := "StateTransition"
	if smType == "EXPRESS" {
		usageType = "ExpressTransition"
	}

	return []clouds.CostUnit{
		clouds.NewCostUnit("transitions", "1k-transitions", transitions/1000, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSStepFunctions",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": usageType,
				"type":      smType,
			},
		}, 0.5),
	}, nil
}
