package extvm

import (
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"github.com/googleapis/gax-go/v2"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
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
	config.Config.ProjectID = "p_extension_gcp"
	config.Config.DiscoveryAttributesExcludesVM = []string{"gcp-vm.label.tag1"}
	id := uint64(42)
	instances := []*computepb.Instance{
		{
			Name:               extutil.Ptr("myVm"),
			Id:                 &id,
			Hostname:           extutil.Ptr("asd"),
			Description:        extutil.Ptr("description"),
			CpuPlatform:        extutil.Ptr("intel"),
			MachineType:        extutil.Ptr("fat"),
			SourceMachineImage: extutil.Ptr("18.04.5 LTS"),
			Status:             extutil.Ptr("top"),
			StatusMessage:      extutil.Ptr("top status"),
			Zone:               extutil.Ptr("/asd/us-east1-a"),
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
						Key:   extutil.Ptr("cluster-name"),
						Value: extutil.Ptr("my_cluster"),
					},
					{
						Key:   extutil.Ptr("cluster-location"),
						Value: extutil.Ptr("us-east1-a"),
					},
				},
			},
		},
	}

	// When
	targets := instancesToTargets(instances)

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
	assert.Equal(t, []string{"/asd/us-east1-a"}, target.Attributes["gcp-vm.zone-url"])
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

func TestMissingProjectId(t *testing.T) {
	// Given
	mockedApi := new(gcpResourceGraphClientMock)
	config.Config.ProjectID = ""

	// When
	_, err := getAllVirtualMachinesInstances(context.Background(), mockedApi)

	// Then
	assert.Equal(t, err.Error(), "project id is not set")
}
