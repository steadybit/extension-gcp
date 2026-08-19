package extvm

import (
	"context"
	"testing"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type gcpResourceGraphClientMock struct {
	mock.Mock
}

func (m *gcpResourceGraphClientMock) AggregatedList(ctx context.Context, req *computepb.AggregatedListInstancesRequest, opts ...gax.CallOption) *compute.InstancesScopedListPairIterator {
	args := m.Called(ctx, req, opts)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*compute.InstancesScopedListPairIterator)
}

func TestInstancesToTargets(t *testing.T) {
	// Given
	config.Config.DiscoveryAttributesExcludesVM = []string{"gcp-vm.label.tag1"}
	id := uint64(42)
	instances := []*computepb.Instance{
		{
			Name:               new("myVm"),
			Id:                 &id,
			Hostname:           new("asd"),
			Description:        new("description"),
			CpuPlatform:        new("intel"),
			MachineType:        new("fat"),
			SourceMachineImage: new("18.04.5 LTS"),
			Status:             new("top"),
			StatusMessage:      new("top status"),
			Zone:               new("/asd/us-east1-a"),
			Tags: &computepb.Tags{
				Items: []string{"Tags1", "Tags2"},
			},
			Labels: map[string]string{
				"tag1": "Value1",
				"tag2": "Value2",
			},
			Metadata: &computepb.Metadata{
				Items: []*computepb.Items{
					{
						Key:   new("cluster-name"),
						Value: new("my_cluster"),
					},
					{
						Key:   new("cluster-location"),
						Value: new("us-east1-a"),
					},
				},
			},
		},
	}

	// When
	targets := instancesToTargets(instances, "p_extension_gcp")

	// Then
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, TargetIDVM, target.TargetType)
	assert.Equal(t, "myVm", target.Label)
	assert.Equal(t, []string{"myVm"}, target.Attributes["gcp-vm.name"])
	assert.Equal(t, []string{"42"}, target.Attributes["gcp-vm.id"])
	assert.Equal(t, []string{"asd"}, target.Attributes["gcp-vm.hostname"])
	assert.Equal(t, []string{"description"}, target.Attributes["gcp-vm.description"])
	assert.Equal(t, []string{"intel"}, target.Attributes["gcp-vm.cpu-platform"])
	assert.Equal(t, []string{"fat"}, target.Attributes["gcp-vm.machine-type"])
	assert.Equal(t, []string{"18.04.5 LTS"}, target.Attributes["gcp-vm.source-machine-image"])
	assert.Equal(t, []string{"top"}, target.Attributes["gcp-vm.status"])
	assert.Equal(t, []string{"top status"}, target.Attributes["gcp-vm.status-message"])
	assert.Equal(t, []string{"/asd/us-east1-a"}, target.Attributes["gcp.zone-url"])
	assert.Equal(t, []string{"us-east1-a"}, target.Attributes["gcp.zone"])
	assert.Equal(t, []string{"p_extension_gcp"}, target.Attributes["gcp.project.id"])
	assert.Equal(t, []string{"my_cluster"}, target.Attributes["gcp-kubernetes-engine.cluster.name"])
	assert.Equal(t, []string{"us-east1-a"}, target.Attributes["gcp-kubernetes-engine.cluster.location"])
	assert.Equal(t, []string{"Tags1,Tags2"}, target.Attributes["gcp-vm.tags"])
	assert.Equal(t, []string{"Value2"}, target.Attributes["gcp-vm.label.tag2"])
	assert.NotContains(t, target.Attributes, "gcp-vm.label.tag1")
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetVMToXEnrichmentRuleCopiesLabelsAndInstanceId(t *testing.T) {
	rule := getVMToXEnrichmentRule("com.steadybit.extension_kubernetes.kubernetes-deployment")

	assert.Equal(t, "com.steadybit.extension_gcp.vm.instance-to-com.steadybit.extension_kubernetes.kubernetes-deployment", rule.Id)
	assert.Equal(t, []discovery_kit_api.Attribute{
		{Matcher: discovery_kit_api.StartsWith, Name: attrPrefixVmLabel},
		{Matcher: discovery_kit_api.Equals, Name: attrVmID},
		{Matcher: discovery_kit_api.Equals, Name: attrZone},
		{Matcher: discovery_kit_api.Equals, Name: attrRegion},
		{Matcher: discovery_kit_api.Equals, Name: attrProjectID},
	}, rule.Attributes)
}

func TestGetRegion(t *testing.T) {
	for _, tt := range []struct {
		zoneUrl string
		want    string
	}{
		{zoneUrl: "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a", want: "us-central1"},
		{zoneUrl: "https://www.googleapis.com/compute/v1/projects/p/zones/europe-west1-b", want: "europe-west1"},
		{zoneUrl: "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1", want: ""},
		{zoneUrl: "", want: ""},
	} {
		t.Run(tt.zoneUrl, func(t *testing.T) {
			assert.Equal(t, tt.want, getRegion(&computepb.Instance{Zone: extutil.Ptr(tt.zoneUrl)}))
		})
	}
}
