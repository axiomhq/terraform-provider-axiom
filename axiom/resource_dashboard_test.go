package axiom

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

func TestNormalizeDashboardString(t *testing.T) {
	got, err := normalizeDashboardString(`{"name":"n","schemaVersion":2}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("expected valid normalized JSON, got %v", err)
	}
}

func TestNormalizeDashboardString_InvalidJSON(t *testing.T) {
	_, err := normalizeDashboardString(`{"name":`)
	if err == nil {
		t.Fatal("expected JSON validation error")
	}
}

func TestDashboardUpsertPayloadFromModel_UpdateUsesStateUIDAndVersion(t *testing.T) {
	plan := DashboardResourceModel{
		Dashboard: types.StringValue(`{"name":"dashboard"}`),
		Overwrite: types.BoolValue(false),
	}

	payload, uid, diags := dashboardUpsertPayloadFromModel(plan, "uid_from_state", 7, false)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	if uid != "uid_from_state" {
		t.Fatalf("expected uid fallback to state, got %q", uid)
	}
	if payload.Version != 7 {
		t.Fatalf("expected payload version 7, got %d", payload.Version)
	}
}

func TestDashboardWriteErrorDiagnostics_VersionMismatch(t *testing.T) {
	diagnostics := diag.Diagnostics{}
	addDashboardWriteErrorDiagnostics(&diagnostics, axiom.HTTPError{Status: 412, Message: "dashboard version mismatch"}, "uid1", 3)

	if !diagnostics.HasError() {
		t.Fatal("expected version mismatch diagnostics")
	}

	got := diagnostics[0].Detail()
	if !strings.Contains(got, "overwrite = true") {
		t.Fatalf("expected actionable mismatch message, got %q", got)
	}
}
