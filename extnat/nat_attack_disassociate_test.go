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
	"google.golang.org/protobuf/proto"
)

func natPrepareReq(attrs map[string][]string) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{Attributes: attrs}),
	})
}

var validNatAttrs = map[string][]string{
	"gcp.project.id":       {"proj-a"},
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

func TestFindNat_Found(t *testing.T) {
	target := &computepb.RouterNat{
		Name: ptr("target-nat"),
		Subnetworks: []*computepb.RouterNatSubnetworkToNat{
			{Name: ptr("subnet-a"), SourceIpRangesToNat: []string{"ALL_IP_RANGES"}},
			{Name: ptr("subnet-b"), SecondaryIpRangeNames: []string{"secondary-1"}},
		},
	}
	router := &computepb.Router{
		Nats: []*computepb.RouterNat{
			{Name: ptr("other-nat"), Subnetworks: []*computepb.RouterNatSubnetworkToNat{{Name: ptr("other-subnet")}}},
			target,
		},
	}
	nat := findNat(router, "target-nat")
	require.NotNil(t, nat)
	assert.Equal(t, "target-nat", nat.GetName())
	assert.Len(t, nat.GetSubnetworks(), 2)
}

func TestFindNat_NotFound(t *testing.T) {
	router := &computepb.Router{
		Nats: []*computepb.RouterNat{{Name: ptr("other-nat")}},
	}
	assert.Nil(t, findNat(router, "missing"))
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

// TestNatSnapshotRoundTrip verifies proto.Marshal → proto.Unmarshal round-trips
// preserve every NAT field the attack cares about — otherwise Stop would
// restore a subtly-different NAT than Prepare captured.
func TestNatSnapshotRoundTrip(t *testing.T) {
	original := &computepb.RouterNat{
		Name:                          ptr("prod-nat"),
		SourceSubnetworkIpRangesToNat: ptr("LIST_OF_SUBNETWORKS"),
		NatIpAllocateOption:           ptr("MANUAL_ONLY"),
		NatIps:                        []string{"projects/p/regions/r/addresses/nat-ip-1"},
		Subnetworks: []*computepb.RouterNatSubnetworkToNat{
			{Name: ptr("subnet-a"), SourceIpRangesToNat: []string{"ALL_IP_RANGES"}},
			{Name: ptr("subnet-b"), SecondaryIpRangeNames: []string{"secondary-1"}},
		},
		MinPortsPerVm: ptrI32(64),
		LogConfig:     &computepb.RouterNatLogConfig{Enable: ptrBool(true)},
	}

	blob, err := proto.Marshal(original)
	require.NoError(t, err)
	require.NotEmpty(t, blob)

	restored := &computepb.RouterNat{}
	require.NoError(t, proto.Unmarshal(blob, restored))

	// proto.Equal ignores nil vs empty-slice subtleties and does the deep
	// comparison we actually care about.
	assert.True(t, proto.Equal(original, restored), "restored NAT differs from original")
	// Belt-and-suspenders spot checks in case proto.Equal is too lenient.
	assert.Equal(t, "prod-nat", restored.GetName())
	assert.Equal(t, "LIST_OF_SUBNETWORKS", restored.GetSourceSubnetworkIpRangesToNat())
	assert.Len(t, restored.GetSubnetworks(), 2)
	assert.Equal(t, "subnet-a", restored.GetSubnetworks()[0].GetName())
	assert.Equal(t, int32(64), restored.GetMinPortsPerVm())
}

func ptrBool(b bool) *bool { return &b }
