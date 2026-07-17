/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extcloudsql

import (
	"context"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudSqlFailover_Prepare_Success(t *testing.T) {
	a := &cloudSqlFailoverAttack{}
	state := CloudSqlFailoverState{}
	req := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"gcp.project.id":                  {"proj-a"},
				"gcp.cloudsql.instance.name":      {"primary"},
				"gcp.cloudsql.availability-type":  {"REGIONAL"},
			},
		}),
	})
	res, err := a.Prepare(context.Background(), &state, req)
	require.NoError(t, err)
	assert.Nil(t, res)
	assert.Equal(t, "proj-a", state.ProjectID)
	assert.Equal(t, "primary", state.InstanceName)
}

func TestCloudSqlFailover_Prepare_MissingProjectID(t *testing.T) {
	a := &cloudSqlFailoverAttack{}
	state := CloudSqlFailoverState{}
	req := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"gcp.cloudsql.instance.name":     {"primary"},
				"gcp.cloudsql.availability-type": {"REGIONAL"},
			},
		}),
	})
	_, err := a.Prepare(context.Background(), &state, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gcp.project.id")
}

func TestCloudSqlFailover_Prepare_NonRegionalRejected(t *testing.T) {
	a := &cloudSqlFailoverAttack{}
	state := CloudSqlFailoverState{}
	req := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"gcp.project.id":                 {"proj-a"},
				"gcp.cloudsql.instance.name":     {"primary"},
				"gcp.cloudsql.availability-type": {"ZONAL"},
			},
		}),
	})
	_, err := a.Prepare(context.Background(), &state, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "REGIONAL")
}

func TestCloudSqlFailover_Prepare_MissingAvailabilityType(t *testing.T) {
	a := &cloudSqlFailoverAttack{}
	state := CloudSqlFailoverState{}
	req := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"gcp.project.id":             {"proj-a"},
				"gcp.cloudsql.instance.name": {"primary"},
			},
		}),
	})
	_, err := a.Prepare(context.Background(), &state, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "REGIONAL")
}

func TestCloudSqlFailover_Describe(t *testing.T) {
	a := &cloudSqlFailoverAttack{}
	desc := a.Describe()
	assert.Equal(t, InstanceFailoverActionId, desc.Id)
	assert.Equal(t, TargetIDInstance, desc.TargetSelection.TargetType)
	assert.Equal(t, CloudSqlFailoverState{}, a.NewEmptyState())
}

func TestCloudSqlFailover_NewInstanceFailoverAction(t *testing.T) {
	a := NewInstanceFailoverAction()
	assert.NotNil(t, a)
}

func TestMustHave(t *testing.T) {
	attrs := map[string][]string{
		"present": {"value"},
		"empty":   {},
	}
	assert.Equal(t, "value", mustHave(attrs, "present"))
	assert.Equal(t, "", mustHave(attrs, "missing"))
	assert.Equal(t, "", mustHave(attrs, "empty"))
}
