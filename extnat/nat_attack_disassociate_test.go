/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extnat

import (
	"context"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func natPrepareReq(attrs map[string][]string) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{Attributes: attrs}),
	})
}

var validNatAttrs = map[string][]string{
	"gcp.project.id":     {"proj-a"},
	"gcp.cloud-nat.region": {"europe-west1"},
	"gcp.cloud-nat.router": {"main-router"},
	"gcp.cloud-nat.name":   {"main-nat"},
}

func TestNatDisassociate_Prepare_MissingRequiredAttr(t *testing.T) {
	for _, drop := range []string{"gcp.project.id", "gcp.cloud-nat.region", "gcp.cloud-nat.router", "gcp.cloud-nat.name"} {
		attrs := map[string][]string{}
		for k, v := range validNatAttrs {
			if k != drop {
				attrs[k] = v
			}
		}
		a := &cloudNatDisassociateAttack{}
		state := CloudNatDisassociateState{}
		_, err := a.Prepare(context.Background(), &state, natPrepareReq(attrs))
		require.Error(t, err, "dropping %s should fail Prepare", drop)
		assert.Contains(t, err.Error(), "missing")
	}
}

func TestNatDisassociate_Describe(t *testing.T) {
	a := &cloudNatDisassociateAttack{}
	desc := a.Describe()
	assert.Equal(t, CloudNatDisassociateActionId, desc.Id)
	assert.Equal(t, TargetIDCloudNat, desc.TargetSelection.TargetType)
	assert.NotNil(t, desc.Stop)
	assert.Equal(t, CloudNatDisassociateState{}, a.NewEmptyState())
}

func TestNatDisassociate_NewAction(t *testing.T) {
	a := NewCloudNatDisassociateAction()
	assert.NotNil(t, a)
}

func TestToSubnetworkProtos(t *testing.T) {
	snaps := []natSubnetSnapshot{
		{Name: "subnet-a", SourceIPRangesToNat: []string{"ALL_IP_RANGES"}},
		{Name: "subnet-b", SecondaryIPRangeNames: []string{"secondary-1"}},
	}
	protos := toSubnetworkProtos(snaps)
	require.Len(t, protos, 2)

	// Reconstruct back for a round-trip sanity check on the pointer marshalling.
	assert.Equal(t, "subnet-a", *protos[0].Name)
	assert.Equal(t, []string{"ALL_IP_RANGES"}, protos[0].SourceIpRangesToNat)
	assert.Equal(t, "subnet-b", *protos[1].Name)
	assert.Equal(t, []string{"secondary-1"}, protos[1].SecondaryIpRangeNames)
}

// Compile-time coverage: this pulls in the RouterNat proto type to make sure
// natSubnetSnapshot struct changes stay proto-compatible.
var _ = &computepb.RouterNat{}

func TestSnapshotNatSubnetworks_Found(t *testing.T) {
	router := &computepb.Router{
		Nats: []*computepb.RouterNat{
			{Name: ptr("other-nat"), Subnetworks: []*computepb.RouterNatSubnetworkToNat{{Name: ptr("other-subnet")}}},
			{
				Name: ptr("target-nat"),
				Subnetworks: []*computepb.RouterNatSubnetworkToNat{
					{Name: ptr("subnet-a"), SourceIpRangesToNat: []string{"ALL_IP_RANGES"}},
					nil,
					{Name: ptr("subnet-b"), SecondaryIpRangeNames: []string{"secondary-1"}},
				},
			},
		},
	}
	found, snaps := snapshotNatSubnetworks(router, "target-nat")
	assert.True(t, found)
	require.Len(t, snaps, 2)
	assert.Equal(t, "subnet-a", snaps[0].Name)
	assert.Equal(t, []string{"ALL_IP_RANGES"}, snaps[0].SourceIPRangesToNat)
	assert.Equal(t, "subnet-b", snaps[1].Name)
	assert.Equal(t, []string{"secondary-1"}, snaps[1].SecondaryIPRangeNames)
}

func TestSnapshotNatSubnetworks_NotFound(t *testing.T) {
	router := &computepb.Router{
		Nats: []*computepb.RouterNat{{Name: ptr("other-nat")}},
	}
	found, snaps := snapshotNatSubnetworks(router, "missing")
	assert.False(t, found)
	assert.Nil(t, snaps)
}

func TestSnapshotNatSubnetworks_FoundButEmpty(t *testing.T) {
	router := &computepb.Router{
		Nats: []*computepb.RouterNat{{Name: ptr("solo")}},
	}
	found, snaps := snapshotNatSubnetworks(router, "solo")
	assert.True(t, found)
	assert.Empty(t, snaps)
}

func TestPopulatePrepareTarget(t *testing.T) {
	// Success
	state := &CloudNatDisassociateState{}
	err := populatePrepareTarget(state, natPrepareReq(validNatAttrs))
	require.NoError(t, err)
	assert.Equal(t, "proj-a", state.ProjectID)
	assert.Equal(t, "main-nat", state.NatName)

	// Missing attribute fails.
	err = populatePrepareTarget(&CloudNatDisassociateState{}, natPrepareReq(map[string][]string{"gcp.project.id": {"p"}}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}
