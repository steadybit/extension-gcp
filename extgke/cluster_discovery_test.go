/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extgke

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestClassifyLocation(t *testing.T) {
	assert.Equal(t, "zonal", classifyLocation("us-central1-a"))
	assert.Equal(t, "zonal", classifyLocation("europe-west1-b"))
	assert.Equal(t, "regional", classifyLocation("us-central1"))
	assert.Equal(t, "regional", classifyLocation("europe-west1"))
	// No dash at all -> regional (default)
	assert.Equal(t, "regional", classifyLocation("nowhere"))
}

func TestToClusterTarget_Populated(t *testing.T) {
	c := &containerpb.Cluster{
		Name:                 "my-cluster",
		Location:             "us-central1",
		CurrentMasterVersion: "1.28.3-gke.1286000",
		Status:               containerpb.Cluster_RUNNING,
		ReleaseChannel:       &containerpb.ReleaseChannel{Channel: containerpb.ReleaseChannel_REGULAR},
		LoggingService:       "logging.googleapis.com/kubernetes",
		MonitoringService:    "monitoring.googleapis.com/kubernetes",
		Network:              "default",
		Subnetwork:           "default-subnet",
		Locations:            []string{"us-central1-b", "us-central1-a"},
		PrivateClusterConfig: &containerpb.PrivateClusterConfig{
			EnablePrivateNodes:    true,
			EnablePrivateEndpoint: false,
		},
		MasterAuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
			Enabled: true,
			CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
				{CidrBlock: "10.0.0.0/8"},
				{CidrBlock: ""},
			},
		},
		WorkloadIdentityConfig: &containerpb.WorkloadIdentityConfig{
			WorkloadPool: "proj-a.svc.id.goog",
		},
		ShieldedNodes:       &containerpb.ShieldedNodes{Enabled: true},
		BinaryAuthorization: &containerpb.BinaryAuthorization{EvaluationMode: containerpb.BinaryAuthorization_PROJECT_SINGLETON_POLICY_ENFORCE},
		ResourceLabels:      map[string]string{"env": "prod"},
	}
	target := toClusterTarget(c, "proj-a")

	assert.Equal(t, TargetIDCluster, target.TargetType)
	assert.Equal(t, "my-cluster", target.Label)
	assert.Equal(t, "projects/proj-a/locations/us-central1/clusters/my-cluster", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"my-cluster"}, target.Attributes[attrClusterName])
	assert.Equal(t, []string{"us-central1"}, target.Attributes[attrClusterLocation])
	assert.Equal(t, []string{"regional"}, target.Attributes[attrClusterLocationType])
	assert.Equal(t, []string{"my-cluster"}, target.Attributes[attrK8sClusterName])
	assert.Equal(t, []string{"1.28.3-gke.1286000"}, target.Attributes[attrClusterKubernetesVersion])
	assert.Equal(t, []string{"RUNNING"}, target.Attributes["gcp.gke.cluster.status"])
	assert.Equal(t, []string{"REGULAR"}, target.Attributes[attrClusterReleaseChannel])
	assert.Equal(t, []string{"logging.googleapis.com/kubernetes"}, target.Attributes[attrClusterLoggingService])
	assert.Equal(t, []string{"monitoring.googleapis.com/kubernetes"}, target.Attributes[attrClusterMonitoringService])
	assert.Equal(t, []string{"default"}, target.Attributes[attrClusterNetwork])
	assert.Equal(t, []string{"default-subnet"}, target.Attributes[attrClusterSubnetwork])
	assert.Equal(t, []string{"us-central1-a", "us-central1-b"}, target.Attributes["gcp.gke.cluster.node-locations"])
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterPrivateCluster])
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterMasterAuthorizedNetsEnabled])
	assert.Equal(t, []string{"10.0.0.0/8"}, target.Attributes[attrClusterMasterAuthorizedNetsCidrs])
	// MAN enabled AND restricted -> API not open to internet
	assert.Equal(t, []string{"false"}, target.Attributes[attrClusterApiServerOpenToInternet])
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterWorkloadIdentityEnabled])
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterShieldedNodesEnabled])
	assert.Equal(t, []string{"PROJECT_SINGLETON_POLICY_ENFORCE"}, target.Attributes[attrClusterBinaryAuthEvalMode])
	assert.Equal(t, []string{"prod"}, target.Attributes["gcp.gke.cluster.label.env"])
}

func TestToClusterTarget_Sparse(t *testing.T) {
	c := &containerpb.Cluster{
		Name:     "sparse",
		Location: "us-central1-a",
	}
	target := toClusterTarget(c, "proj-a")

	assert.Equal(t, "sparse", target.Label)
	assert.Equal(t, []string{"zonal"}, target.Attributes[attrClusterLocationType])
	// PrivateClusterConfig nil -> private-cluster is false
	assert.Equal(t, []string{"false"}, target.Attributes[attrClusterPrivateCluster])
	// MAN disabled: no CIDRs -> not restricted -> API open to internet true
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterApiServerOpenToInternet])
	// Absent: kubernetes version, status, release channel, logging, monitoring
	assert.NotContains(t, target.Attributes, attrClusterKubernetesVersion)
	assert.NotContains(t, target.Attributes, "gcp.gke.cluster.status")
	assert.NotContains(t, target.Attributes, attrClusterReleaseChannel)
	assert.NotContains(t, target.Attributes, attrClusterLoggingService)
	// WorkloadIdentity absent -> false
	assert.Equal(t, []string{"false"}, target.Attributes[attrClusterWorkloadIdentityEnabled])
	assert.Equal(t, []string{"false"}, target.Attributes[attrClusterShieldedNodesEnabled])
}

func TestToClusterTarget_ManDisabledButHasCidrs_NotMisreportedAsRestricted(t *testing.T) {
	// MAN disabled AND has non-wildcard leftover CIDR: must NOT be reported as restricted (API open = true)
	c := &containerpb.Cluster{
		Name:     "c",
		Location: "us-central1",
		MasterAuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
			Enabled: false,
			CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
				{CidrBlock: "10.0.0.0/8"},
			},
		},
	}
	target := toClusterTarget(c, "proj-a")
	assert.Equal(t, []string{"false"}, target.Attributes[attrClusterMasterAuthorizedNetsEnabled])
	// MAN disabled -> not restricted -> API server open to internet
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterApiServerOpenToInternet])
}

func TestToClusterTarget_ManEnabledOnlyWildcard_ReportsOpen(t *testing.T) {
	c := &containerpb.Cluster{
		Name:     "c",
		Location: "us-central1",
		MasterAuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
			Enabled: true,
			CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
				{CidrBlock: "0.0.0.0/0"},
			},
		},
	}
	target := toClusterTarget(c, "proj-a")
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterMasterAuthorizedNetsEnabled])
	// Only 0.0.0.0/0 -> not restricted -> open true
	assert.Equal(t, []string{"true"}, target.Attributes[attrClusterApiServerOpenToInternet])
}

func TestToClusterTarget_PrivateEndpoint_ReportsNotOpen(t *testing.T) {
	c := &containerpb.Cluster{
		Name:     "c",
		Location: "us-central1",
		PrivateClusterConfig: &containerpb.PrivateClusterConfig{
			EnablePrivateNodes:    true,
			EnablePrivateEndpoint: true,
		},
	}
	target := toClusterTarget(c, "proj-a")
	assert.Equal(t, []string{"false"}, target.Attributes[attrClusterApiServerOpenToInternet])
}

func TestClusterDiscovery_DescribeMethods(t *testing.T) {
	d := &clusterDiscovery{}
	assert.Equal(t, TargetIDCluster, d.Describe().Id)
	assert.Equal(t, TargetIDCluster, d.DescribeTarget().Id)
	assert.NotEmpty(t, d.DescribeAttributes())
	rules := d.DescribeEnrichmentRules()
	assert.NotEmpty(t, rules)
	// All target types should be covered
	assert.Len(t, rules, len(gkeEnrichmentTargetTypes))
}

func TestNewClusterDiscovery(t *testing.T) {
	assert.NotNil(t, NewClusterDiscovery())
}

func TestGkeClusterToK8sEnrichmentRule(t *testing.T) {
	r := gkeClusterToK8sEnrichmentRule("com.steadybit.extension_kubernetes.kubernetes-node")
	assert.Contains(t, r.Id, "cluster-to-com.steadybit.extension_kubernetes.kubernetes-node")
	assert.Equal(t, TargetIDCluster, r.Src.Type)
	assert.Equal(t, "com.steadybit.extension_kubernetes.kubernetes-node", r.Dest.Type)
	assert.NotEmpty(t, r.Attributes)
}

// clusterManagerApiMock satisfies clusterManagerApi.
type clusterManagerApiMock struct {
	mock.Mock
}

func (m *clusterManagerApiMock) ListClusters(ctx context.Context, req *containerpb.ListClustersRequest, opts ...gax.CallOption) (*containerpb.ListClustersResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*containerpb.ListClustersResponse), args.Error(1)
}

func (m *clusterManagerApiMock) ListNodePools(ctx context.Context, req *containerpb.ListNodePoolsRequest, opts ...gax.CallOption) (*containerpb.ListNodePoolsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*containerpb.ListNodePoolsResponse), args.Error(1)
}

func TestGetAllClusters(t *testing.T) {
	m := &clusterManagerApiMock{}
	m.On("ListClusters", mock.Anything, mock.Anything).Return(&containerpb.ListClustersResponse{
		Clusters: []*containerpb.Cluster{
			{Name: "c1", Location: "us-central1"},
			{Name: "c2", Location: "us-central1-a"},
		},
	}, nil)

	targets, err := getAllClusters(context.Background(), m, "proj-a")
	require.NoError(t, err)
	assert.Len(t, targets, 2)
	m.AssertExpectations(t)
}

func TestGetAllClusters_Error(t *testing.T) {
	m := &clusterManagerApiMock{}
	m.On("ListClusters", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	_, err := getAllClusters(context.Background(), m, "proj-a")
	require.Error(t, err)
}
