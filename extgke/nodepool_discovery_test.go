/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extgke

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestToNodePoolTarget_Populated(t *testing.T) {
	cluster := &containerpb.Cluster{Name: "my-cluster", Location: "us-central1"}
	np := &containerpb.NodePool{
		Name:    "np-1",
		Version: "1.28.3-gke.1286000",
		Status:  containerpb.NodePool_RUNNING,
		Config: &containerpb.NodeConfig{
			MachineType: "e2-medium",
			DiskType:    "pd-standard",
			DiskSizeGb:  100,
			ImageType:   "COS_CONTAINERD",
			Preemptible: false,
			Spot:        true,
			Labels:      map[string]string{"team": "core"},
		},
		Autoscaling: &containerpb.NodePoolAutoscaling{
			Enabled:      true,
			MinNodeCount: 1,
			MaxNodeCount: 5,
		},
		MaxPodsConstraint: &containerpb.MaxPodsConstraint{MaxPodsPerNode: 110},
		Management:        &containerpb.NodeManagement{AutoUpgrade: true, AutoRepair: true},
		UpgradeSettings:   &containerpb.NodePool_UpgradeSettings{MaxSurge: 1, MaxUnavailable: 0},
		Locations:         []string{"us-central1-b", "us-central1-a"},
		InstanceGroupUrls: []string{
			"https://www.googleapis.com/compute/v1/projects/proj-a/zones/us-central1-b/instanceGroupManagers/gke-np-b",
			"https://www.googleapis.com/compute/v1/projects/proj-a/zones/us-central1-a/instanceGroupManagers/gke-np-a",
		},
	}
	target := toNodePoolTarget(np, cluster, "proj-a")

	assert.Equal(t, TargetIDNodePool, target.TargetType)
	assert.Equal(t, "my-cluster/np-1", target.Label)
	assert.Equal(t, "projects/proj-a/locations/us-central1/clusters/my-cluster/nodePools/np-1", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes["gcp.project.id"])
	assert.Equal(t, []string{"my-cluster"}, target.Attributes[attrClusterName])
	assert.Equal(t, []string{"my-cluster"}, target.Attributes["k8s.cluster-name"])
	assert.Equal(t, []string{"us-central1"}, target.Attributes["gcp.gke.cluster.location"])
	assert.Equal(t, []string{"np-1"}, target.Attributes["gcp.gke.nodepool.name"])
	assert.Equal(t, []string{"1.28.3-gke.1286000"}, target.Attributes[attrNodePoolKubernetesVersion])
	assert.Equal(t, []string{"RUNNING"}, target.Attributes["gcp.gke.nodepool.status"])
	assert.Equal(t, []string{"e2-medium"}, target.Attributes[attrNodePoolMachineType])
	assert.Equal(t, []string{"pd-standard"}, target.Attributes["gcp.gke.nodepool.disk-type"])
	assert.Equal(t, []string{"100"}, target.Attributes["gcp.gke.nodepool.disk-size-gb"])
	assert.Equal(t, []string{"COS_CONTAINERD"}, target.Attributes["gcp.gke.nodepool.image-type"])
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.gke.nodepool.preemptible"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.gke.nodepool.spot"])
	assert.Equal(t, []string{"true"}, target.Attributes[attrNodePoolAutoscalingEnabled])
	assert.Equal(t, []string{"1"}, target.Attributes["gcp.gke.nodepool.autoscaling.min-node-count"])
	assert.Equal(t, []string{"5"}, target.Attributes["gcp.gke.nodepool.autoscaling.max-node-count"])
	assert.Equal(t, []string{"110"}, target.Attributes["gcp.gke.nodepool.max-pods-per-node"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.gke.nodepool.management.auto-upgrade"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.gke.nodepool.management.auto-repair"])
	assert.Equal(t, []string{"1"}, target.Attributes["gcp.gke.nodepool.upgrade-settings.max-surge"])
	assert.Equal(t, []string{"0"}, target.Attributes["gcp.gke.nodepool.upgrade-settings.max-unavailable"])
	assert.Equal(t, []string{"us-central1-a", "us-central1-b"}, target.Attributes["gcp.gke.nodepool.locations"])
	// Sorted URLs
	urls := target.Attributes["gcp.gke.nodepool.instance-group-urls"]
	assert.Len(t, urls, 2)
	assert.Equal(t, urls[0], "https://www.googleapis.com/compute/v1/projects/proj-a/zones/us-central1-a/instanceGroupManagers/gke-np-a")
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.gke.nodepool.label.team"])
}

func TestToNodePoolTarget_Sparse(t *testing.T) {
	cluster := &containerpb.Cluster{Name: "my-cluster", Location: "us-central1"}
	np := &containerpb.NodePool{
		Name: "empty",
	}
	target := toNodePoolTarget(np, cluster, "proj-a")

	assert.Equal(t, "my-cluster/empty", target.Label)
	// Autoscaling absent -> false
	assert.Equal(t, []string{"false"}, target.Attributes[attrNodePoolAutoscalingEnabled])
	assert.NotContains(t, target.Attributes, "gcp.gke.nodepool.autoscaling.min-node-count")
	assert.NotContains(t, target.Attributes, "gcp.gke.nodepool.status")
	assert.NotContains(t, target.Attributes, attrNodePoolMachineType)
	assert.NotContains(t, target.Attributes, "gcp.gke.nodepool.image-type")
	assert.NotContains(t, target.Attributes, "gcp.gke.nodepool.max-pods-per-node")
	assert.NotContains(t, target.Attributes, "gcp.gke.nodepool.management.auto-upgrade")
	assert.NotContains(t, target.Attributes, "gcp.gke.nodepool.locations")
}

func TestNodePoolDiscovery_DescribeMethods(t *testing.T) {
	d := &nodePoolDiscovery{}
	assert.Equal(t, TargetIDNodePool, d.Describe().Id)
	assert.Equal(t, TargetIDNodePool, d.DescribeTarget().Id)
	assert.NotEmpty(t, d.DescribeAttributes())
}

func TestNewNodePoolDiscovery(t *testing.T) {
	assert.NotNil(t, NewNodePoolDiscovery())
}

func TestGetAllNodePools(t *testing.T) {
	m := &clusterManagerApiMock{}
	m.On("ListClusters", mock.Anything, mock.Anything).Return(&containerpb.ListClustersResponse{
		Clusters: []*containerpb.Cluster{
			{Name: "c1", Location: "us-central1"},
		},
	}, nil)
	m.On("ListNodePools", mock.Anything, mock.Anything).Return(&containerpb.ListNodePoolsResponse{
		NodePools: []*containerpb.NodePool{
			{Name: "np-a"},
			{Name: "np-b"},
		},
	}, nil)

	targets, err := getAllNodePools(context.Background(), m, "proj-a")
	require.NoError(t, err)
	assert.Len(t, targets, 2)
}

func TestGetAllNodePools_ListClustersError(t *testing.T) {
	m := &clusterManagerApiMock{}
	m.On("ListClusters", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllNodePools(context.Background(), m, "proj-a")
	require.Error(t, err)
}

func TestGetAllNodePools_ListNodePoolsErrorSkipped(t *testing.T) {
	m := &clusterManagerApiMock{}
	m.On("ListClusters", mock.Anything, mock.Anything).Return(&containerpb.ListClustersResponse{
		Clusters: []*containerpb.Cluster{
			{Name: "c1", Location: "us-central1"},
		},
	}, nil)
	m.On("ListNodePools", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	// Per-cluster errors are logged and skipped, not returned.
	targets, err := getAllNodePools(context.Background(), m, "proj-a")
	require.NoError(t, err)
	assert.Empty(t, targets)
}
