package axiom

import (
	"encoding/json"
	"errors"
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

func TestNormalizeDashboardRaw(t *testing.T) {
	got, err := normalizeDashboardRaw(json.RawMessage(`{"schemaVersion":2,"name":"dashboard"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(got, `"schemaVersion":2`) {
		t.Fatalf("expected normalized JSON output, got %s", got)
	}
}

func TestNormalizeDashboardRaw_Empty(t *testing.T) {
	_, err := normalizeDashboardRaw(nil)
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestDashboardUpsertPayloadFromModel_CreateWithUID(t *testing.T) {
	plan := DashboardResourceModel{
		UID:       types.StringValue("uid_from_config"),
		Dashboard: types.StringValue(`{"name":"dashboard"}`),
		Overwrite: types.BoolValue(false),
	}

	payload, uid, diags := dashboardUpsertPayloadFromModel(plan, "", 0, true)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	if uid != "uid_from_config" {
		t.Fatalf("expected uid from config, got %q", uid)
	}
	if payload.UID != "uid_from_config" {
		t.Fatalf("expected payload UID from config, got %q", payload.UID)
	}
	if payload.Version != 0 {
		t.Fatalf("expected no version on create, got %d", payload.Version)
	}
}

func TestDashboardUpsertPayloadFromModel_CreateWithoutUID(t *testing.T) {
	plan := DashboardResourceModel{
		Dashboard: types.StringValue(`{"name":"dashboard"}`),
		Overwrite: types.BoolValue(false),
	}

	payload, uid, diags := dashboardUpsertPayloadFromModel(plan, "", 0, true)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	if uid != "" {
		t.Fatalf("expected empty uid to allow server generation, got %q", uid)
	}
	if payload.UID != "" {
		t.Fatalf("expected payload UID to be empty, got %q", payload.UID)
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

func TestDashboardUpsertPayloadFromModel_UpdateForceOverwrite(t *testing.T) {
	plan := DashboardResourceModel{
		UID:       types.StringValue("uid_from_config"),
		Dashboard: types.StringValue(`{"name":"dashboard"}`),
		Overwrite: types.BoolValue(true),
	}

	payload, uid, diags := dashboardUpsertPayloadFromModel(plan, "uid_from_state", 9, false)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	if uid != "uid_from_config" {
		t.Fatalf("expected uid from config to override state uid, got %q", uid)
	}
	if !payload.Overwrite {
		t.Fatal("expected overwrite=true in payload")
	}
	if payload.Version != 0 {
		t.Fatalf("expected version to be omitted on overwrite, got %d", payload.Version)
	}
}

func TestDashboardUpsertPayloadFromModel_UpdateMissingVersion(t *testing.T) {
	plan := DashboardResourceModel{
		UID:       types.StringValue("uid_from_config"),
		Dashboard: types.StringValue(`{"name":"dashboard"}`),
		Overwrite: types.BoolValue(false),
	}

	_, _, diags := dashboardUpsertPayloadFromModel(plan, "uid_from_state", 0, false)
	if !diags.HasError() {
		t.Fatal("expected diagnostics when update version is missing")
	}
}

func TestDashboardUIDFromState(t *testing.T) {
	t.Run("prefers uid", func(t *testing.T) {
		uid := dashboardUIDFromState(DashboardResourceModel{
			UID: types.StringValue("uid-1"),
			ID:  types.StringValue("id-1"),
		})
		if uid != "uid-1" {
			t.Fatalf("expected uid-1, got %q", uid)
		}
	})

	t.Run("falls back to id", func(t *testing.T) {
		uid := dashboardUIDFromState(DashboardResourceModel{ID: types.StringValue("id-1")})
		if uid != "id-1" {
			t.Fatalf("expected id fallback, got %q", uid)
		}
	})
}

func TestFlattenDashboardResource(t *testing.T) {
	in := dashboardResourcePayload{
		UID:       "uid-1",
		ID:        "id-1",
		Version:   5,
		Dashboard: json.RawMessage(`{"name":"dash"}`),
	}

	got, err := flattenDashboardResource(in, types.BoolValue(false))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID.ValueString() != "id-1" {
		t.Fatalf("expected state id from response id, got %q", got.ID.ValueString())
	}
	if got.UID.ValueString() != "uid-1" {
		t.Fatalf("expected state uid from response uid, got %q", got.UID.ValueString())
	}
	if got.Version.ValueInt64() != 5 {
		t.Fatalf("expected version=5, got %d", got.Version.ValueInt64())
	}
}

func TestFlattenDashboardResource_MissingUID(t *testing.T) {
	_, err := flattenDashboardResource(dashboardResourcePayload{ID: "id-1"}, types.BoolValue(false))
	if err == nil {
		t.Fatal("expected error when uid is missing")
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

func TestDashboardWriteErrorDiagnostics_APIError(t *testing.T) {
	diagnostics := diag.Diagnostics{}
	addDashboardWriteErrorDiagnostics(&diagnostics, axiom.HTTPError{Status: 409, Message: "dashboard uid already exists"}, "uid1", 1)

	if !diagnostics.HasError() {
		t.Fatal("expected diagnostics for API error")
	}

	if !strings.Contains(diagnostics[0].Detail(), "status 409") {
		t.Fatalf("expected HTTP status in diagnostics, got %q", diagnostics[0].Detail())
	}
}

func TestDashboardWriteErrorDiagnostics_NonAPIError(t *testing.T) {
	diagnostics := diag.Diagnostics{}
	addDashboardWriteErrorDiagnostics(&diagnostics, errors.New("network error"), "uid1", 1)

	if !diagnostics.HasError() {
		t.Fatal("expected diagnostics for generic error")
	}
	if !strings.Contains(diagnostics[0].Detail(), "network error") {
		t.Fatalf("expected original error message, got %q", diagnostics[0].Detail())
	}
}
