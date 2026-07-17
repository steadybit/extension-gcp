/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extnat

import (
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/stretchr/testify/assert"
)

func TestToNatTarget_Populated(t *testing.T) {
	true_ := true
	router := &computepb.Router{
		Name:     ptr("main-router"),
		SelfLink: ptr("projects/proj-a/regions/europe-west1/routers/main-router"),
		Network:  ptr("projects/proj-a/global/networks/default"),
	}
	nat := &computepb.RouterNat{
		Name:                         ptr("main-nat"),
		SourceSubnetworkIpRangesToNat: ptr("LIST_OF_SUBNETWORKS"),
		NatIpAllocateOption:          ptr("MANUAL_ONLY"),
		NatIps:                       []string{"projects/proj-a/regions/europe-west1/addresses/nat-b", "projects/proj-a/regions/europe-west1/addresses/nat-a"},
		EndpointTypes:                []string{"ENDPOINT_TYPE_SWG", "ENDPOINT_TYPE_VM"},
		Subnetworks: []*computepb.RouterNatSubnetworkToNat{
			{Name: ptr("subnet-b")},
			nil,
			{Name: ptr("subnet-a")},
			{Name: ptr("")},
		},
		MinPortsPerVm: ptrI32(64),
		LogConfig: &computepb.RouterNatLogConfig{
			Enable: &true_,
		},
		EnableDynamicPortAllocation: &true_,
	}

	target := toNatTarget(router, nat, "europe-west1", "proj-a")

	assert.Equal(t, TargetIDCloudNat, target.TargetType)
	assert.Equal(t, "main-router/main-nat", target.Label)
	assert.Equal(t, "projects/proj-a/regions/europe-west1/routers/main-router/nats/main-nat", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"main-nat"}, target.Attributes["gcp.cloud-nat.name"])
	assert.Equal(t, []string{"main-router"}, target.Attributes["gcp.cloud-nat.router"])
	assert.Equal(t, []string{"europe-west1"}, target.Attributes[attrRegion])
	assert.Equal(t, []string{"projects/proj-a/global/networks/default"}, target.Attributes["gcp.cloud-nat.network"])
	assert.Equal(t, []string{"LIST_OF_SUBNETWORKS"}, target.Attributes[attrSourceSubnetworkIpRanges])
	assert.Equal(t, []string{"MANUAL_ONLY"}, target.Attributes["gcp.cloud-nat.nat-ip-allocate-option"])
	// NatIps and EndpointTypes should be sorted.
	assert.Equal(t, []string{"projects/proj-a/regions/europe-west1/addresses/nat-a", "projects/proj-a/regions/europe-west1/addresses/nat-b"}, target.Attributes["gcp.cloud-nat.nat-ips"])
	assert.Equal(t, []string{"ENDPOINT_TYPE_SWG", "ENDPOINT_TYPE_VM"}, target.Attributes["gcp.cloud-nat.endpoint-types"])
	// Subnetworks filtered (nil + empty-name skipped) and sorted.
	assert.Equal(t, []string{"subnet-a", "subnet-b"}, target.Attributes["gcp.cloud-nat.subnetworks"])
	// subnetwork-count matches the filtered list, not the raw slice length.
	assert.Equal(t, []string{"2"}, target.Attributes[attrSubnetworkCount])
	assert.Equal(t, []string{"64"}, target.Attributes["gcp.cloud-nat.min-ports-per-vm"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloud-nat.log-config.enable"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloud-nat.enable-dynamic-port-allocation"])
}

func TestToNatTarget_Sparse(t *testing.T) {
	router := &computepb.Router{Name: ptr("r"), SelfLink: ptr("s")}
	nat := &computepb.RouterNat{Name: ptr("n")}

	target := toNatTarget(router, nat, "europe-west1", "proj-a")

	assert.Equal(t, "r/n", target.Label)
	assert.Equal(t, []string{"0"}, target.Attributes[attrSubnetworkCount])
	assert.NotContains(t, target.Attributes, "gcp.cloud-nat.network")
	assert.NotContains(t, target.Attributes, attrSourceSubnetworkIpRanges)
	assert.NotContains(t, target.Attributes, "gcp.cloud-nat.nat-ips")
	assert.NotContains(t, target.Attributes, "gcp.cloud-nat.subnetworks")
	assert.NotContains(t, target.Attributes, "gcp.cloud-nat.min-ports-per-vm")
	assert.NotContains(t, target.Attributes, "gcp.cloud-nat.log-config.enable")
	assert.NotContains(t, target.Attributes, "gcp.cloud-nat.enable-dynamic-port-allocation")
}

func TestNatDescribeMethods(t *testing.T) {
	d := &natDiscovery{}
	assert.Equal(t, TargetIDCloudNat, d.Describe().Id)
	assert.Equal(t, TargetIDCloudNat, d.DescribeTarget().Id)
	assert.NotEmpty(t, d.DescribeAttributes())
}

func TestNewNatDiscovery(t *testing.T) {
	assert.NotNil(t, NewNatDiscovery())
}

func ptr(s string) *string  { return &s }
func ptrI32(v int32) *int32 { return &v }
