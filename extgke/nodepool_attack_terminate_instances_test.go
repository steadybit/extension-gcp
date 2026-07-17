/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extgke

import (
	"context"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gkePrepareReq(attrs map[string][]string, cfg map[string]interface{}) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{Attributes: attrs}),
		Config: cfg,
	})
}

var validNodePoolAttrs = map[string][]string{
	"gcp.project.id":            {"proj-a"},
	attrClusterName:             {"prod"},
	"gcp.gke.nodepool.name":     {"default-pool"},
	"gcp.gke.cluster.location":  {"europe-west1-b"},
}

func TestNodePoolTerminate_Prepare_MissingRequiredAttr(t *testing.T) {
	for _, drop := range []string{"gcp.project.id", attrClusterName, "gcp.gke.nodepool.name", "gcp.gke.cluster.location"} {
		attrs := map[string][]string{}
		for k, v := range validNodePoolAttrs {
			if k != drop {
				attrs[k] = v
			}
		}
		a := &nodePoolTerminateInstancesAttack{}
		state := NodePoolTerminateInstancesState{}
		_, err := a.Prepare(context.Background(), &state, gkePrepareReq(attrs, map[string]interface{}{"percentage": 33}))
		require.Error(t, err, "dropping %s should fail Prepare", drop)
		assert.Contains(t, err.Error(), "missing")
	}
}

func TestNodePoolTerminate_Prepare_PercentageOutOfRange(t *testing.T) {
	a := &nodePoolTerminateInstancesAttack{}
	state := NodePoolTerminateInstancesState{}

	_, err := a.Prepare(context.Background(), &state, gkePrepareReq(validNodePoolAttrs, map[string]interface{}{"percentage": 0}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "percentage")

	_, err = a.Prepare(context.Background(), &state, gkePrepareReq(validNodePoolAttrs, map[string]interface{}{"percentage": 101}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "percentage")
}

func TestNodePoolTerminate_Prepare_HighImpactGate(t *testing.T) {
	a := &nodePoolTerminateInstancesAttack{}
	state := NodePoolTerminateInstancesState{}
	_, err := a.Prepare(context.Background(), &state, gkePrepareReq(validNodePoolAttrs, map[string]interface{}{"percentage": 75}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Allow percentages above 50%")
}

func TestParseZonalMIGUrl(t *testing.T) {
	// Zonal URL — extract zone and name.
	zone, name, ok := parseZonalMIGUrl("https://www.googleapis.com/compute/v1/projects/proj-a/zones/europe-west1-b/instanceGroupManagers/mig-1")
	assert.True(t, ok)
	assert.Equal(t, "europe-west1-b", zone)
	assert.Equal(t, "mig-1", name)

	// Regional URL — no /zones/ segment.
	_, _, ok = parseZonalMIGUrl("https://www.googleapis.com/compute/v1/projects/proj-a/regions/europe-west1/instanceGroupManagers/rmig")
	assert.False(t, ok)

	// Malformed URL.
	_, _, ok = parseZonalMIGUrl("garbage")
	assert.False(t, ok)
}

func TestNodePoolTerminate_Describe(t *testing.T) {
	a := &nodePoolTerminateInstancesAttack{}
	desc := a.Describe()
	assert.Equal(t, NodePoolTerminateInstancesActionId, desc.Id)
	assert.Equal(t, TargetIDNodePool, desc.TargetSelection.TargetType)
	assert.Equal(t, NodePoolTerminateInstancesState{}, a.NewEmptyState())
}

func TestNodePoolTerminate_NewAction(t *testing.T) {
	a := NewNodePoolTerminateInstancesAction()
	assert.NotNil(t, a)
}

func TestGkeMustHave(t *testing.T) {
	attrs := map[string][]string{
		"present": {"value"},
		"empty":   {},
	}
	assert.Equal(t, "value", mustHave(attrs, "present"))
	assert.Equal(t, "", mustHave(attrs, "missing"))
	assert.Equal(t, "", mustHave(attrs, "empty"))
}
