package axiom

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMergeNotifierStatePagerDutyFallback(t *testing.T) {
	source := NotifierResourceModel{
		Properties: &NotifierProperties{
			Pagerduty: &PagerDutyConfig{
				RoutingKey: types.StringValue("plan-routing-key"),
				Token:      types.StringValue("plan-token"),
			},
		},
	}

	remote := NotifierResourceModel{
		Properties: &NotifierProperties{
			Pagerduty: &PagerDutyConfig{
				RoutingKey: types.StringValue(""),
				Token:      types.StringValue(""),
			},
		},
	}

	merged := mergeNotifierState(remote, source)

	if got := merged.Properties.Pagerduty.RoutingKey.ValueString(); got != "plan-routing-key" {
		t.Fatalf("RoutingKey = %q, expected plan-routing-key", got)
	}

	if got := merged.Properties.Pagerduty.Token.ValueString(); got != "plan-token" {
		t.Fatalf("Token = %q, expected plan-token", got)
	}
}

func TestMergeNotifierStatePagerDutyKeepsRemoteValues(t *testing.T) {
	source := NotifierResourceModel{
		Properties: &NotifierProperties{
			Pagerduty: &PagerDutyConfig{
				RoutingKey: types.StringValue("plan-routing-key"),
				Token:      types.StringValue("plan-token"),
			},
		},
	}

	remote := NotifierResourceModel{
		Properties: &NotifierProperties{
			Pagerduty: &PagerDutyConfig{
				RoutingKey: types.StringValue("api-routing-key"),
				Token:      types.StringValue("api-token"),
			},
		},
	}

	merged := mergeNotifierState(remote, source)

	if got := merged.Properties.Pagerduty.RoutingKey.ValueString(); got != "api-routing-key" {
		t.Fatalf("RoutingKey = %q, expected api-routing-key", got)
	}

	if got := merged.Properties.Pagerduty.Token.ValueString(); got != "api-token" {
		t.Fatalf("Token = %q, expected api-token", got)
	}
}
