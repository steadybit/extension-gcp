/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmig

import (
	"context"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func migPrepareReq(attrs map[string][]string, cfg map[string]interface{}) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{Attributes: attrs}),
		Config: cfg,
	})
}

var validMigTargetAttrs = map[string][]string{
	"gcp.project.id":  {"proj-a"},
	"gcp.mig.scope":   {"zonal"},
	"gcp.mig.location": {"europe-west1-a"},
	"gcp.mig.name":    {"web"},
}

func TestMigDelete_Prepare_MissingRequiredAttr(t *testing.T) {
	for _, drop := range []string{"gcp.project.id", "gcp.mig.scope", "gcp.mig.location", "gcp.mig.name"} {
		attrs := map[string][]string{}
		for k, v := range validMigTargetAttrs {
			if k != drop {
				attrs[k] = v
			}
		}
		a := &migDeleteInstancesAttack{}
		state := MigDeleteInstancesState{}
		_, err := a.Prepare(context.Background(), &state, migPrepareReq(attrs, map[string]interface{}{"percentage": 33}))
		require.Error(t, err, "dropping %s should fail Prepare", drop)
	}
}

func TestMigDelete_Prepare_PercentageOutOfRange(t *testing.T) {
	a := &migDeleteInstancesAttack{}
	state := MigDeleteInstancesState{}

	// 0% invalid
	_, err := a.Prepare(context.Background(), &state, migPrepareReq(validMigTargetAttrs, map[string]interface{}{"percentage": 0}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "percentage")

	// 101% invalid
	_, err = a.Prepare(context.Background(), &state, migPrepareReq(validMigTargetAttrs, map[string]interface{}{"percentage": 101}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "percentage")
}

func TestMigDelete_Prepare_HighImpactGate(t *testing.T) {
	a := &migDeleteInstancesAttack{}
	state := MigDeleteInstancesState{}

	// >50% without confirmHighImpact rejected
	_, err := a.Prepare(context.Background(), &state, migPrepareReq(validMigTargetAttrs, map[string]interface{}{"percentage": 51}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Allow percentages above 50%")
}

func TestMigDelete_Describe(t *testing.T) {
	a := &migDeleteInstancesAttack{}
	desc := a.Describe()
	assert.Equal(t, MigDeleteInstancesActionId, desc.Id)
	assert.Equal(t, TargetIDMig, desc.TargetSelection.TargetType)
	assert.Equal(t, MigDeleteInstancesState{}, a.NewEmptyState())
}

func TestMigDelete_NewAction(t *testing.T) {
	a := NewMigDeleteInstancesAction()
	assert.NotNil(t, a)
}

func TestMigDelete_Start_NoInstancesSelected(t *testing.T) {
	a := &migDeleteInstancesAttack{}
	state := &MigDeleteInstancesState{}
	_, err := a.Start(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No instances selected")
}

func TestMigDelete_ListRunningInstances_UnsupportedScope(t *testing.T) {
	a := &migDeleteInstancesAttack{}
	_, err := a.listRunningInstances(context.Background(), &MigDeleteInstancesState{
		ProjectID: "proj-a", Scope: "unknown", Location: "x", MigName: "y",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}
