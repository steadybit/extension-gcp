/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extgke

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-gcp/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type clusterDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber          = (*clusterDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber       = (*clusterDiscovery)(nil)
	_ discovery_kit_sdk.EnrichmentRulesDescriber = (*clusterDiscovery)(nil)
)

// gkeEnrichmentTargetTypes are the extension-kubernetes target types that get enriched with GKE cluster
// reliability config. Joined on k8s.cluster-name.
var gkeEnrichmentTargetTypes = []string{
	"com.steadybit.extension_kubernetes.kubernetes-deployment",
	"com.steadybit.extension_kubernetes.kubernetes-pod",
	"com.steadybit.extension_kubernetes.kubernetes-statefulset",
	"com.steadybit.extension_kubernetes.kubernetes-daemonset",
	"com.steadybit.extension_kubernetes.kubernetes-node",
	"com.steadybit.extension_kubernetes.argo-rollout",
}

// gkeEnrichmentAttributes are stable, reliability-relevant config attributes copied onto matching
// Kubernetes targets. No labels (high cardinality), no volatile status.
var gkeEnrichmentAttributes = []discovery_kit_api.Attribute{
	{Matcher: discovery_kit_api.Equals, Name: attrProjectID},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterName},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterLocation},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterLocationType},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterKubernetesVersion},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterReleaseChannel},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterPrivateCluster},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterMasterAuthorizedNetsEnabled},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterMasterAuthorizedNetsCidrs},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterApiServerOpenToInternet},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterNetwork},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterSubnetwork},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterWorkloadIdentityEnabled},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterShieldedNodesEnabled},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterBinaryAuthEvalMode},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterLoggingService},
	{Matcher: discovery_kit_api.Equals, Name: attrClusterMonitoringService},
}

func NewClusterDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&clusterDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *clusterDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDCluster,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("60s"),
		},
	}
}

func (d *clusterDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDCluster,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "GKE cluster", Other: "GKE clusters"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: attrClusterKubernetesVersion},
				{Attribute: attrClusterLocation},
				{Attribute: attrProjectID},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *clusterDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: attrClusterName, Label: discovery_kit_api.PluralLabel{One: "GKE cluster name", Other: "GKE cluster names"}},
		{Attribute: attrClusterLocation, Label: discovery_kit_api.PluralLabel{One: "GKE cluster location", Other: "GKE cluster locations"}},
		{Attribute: attrClusterLocationType, Label: discovery_kit_api.PluralLabel{One: "GKE cluster location type", Other: "GKE cluster location types"}},
		{Attribute: attrClusterKubernetesVersion, Label: discovery_kit_api.PluralLabel{One: "GKE Kubernetes version", Other: "GKE Kubernetes versions"}},
		{Attribute: attrClusterReleaseChannel, Label: discovery_kit_api.PluralLabel{One: "GKE release channel", Other: "GKE release channels"}},
		{Attribute: "gcp.gke.cluster.status", Label: discovery_kit_api.PluralLabel{One: "GKE cluster status", Other: "GKE cluster statuses"}},
		{Attribute: attrClusterPrivateCluster, Label: discovery_kit_api.PluralLabel{One: "GKE private cluster", Other: "GKE private clusters"}},
		{Attribute: attrClusterMasterAuthorizedNetsEnabled, Label: discovery_kit_api.PluralLabel{One: "GKE master-authorized-networks", Other: "GKE master-authorized-networks"}},
		{Attribute: attrClusterMasterAuthorizedNetsCidrs, Label: discovery_kit_api.PluralLabel{One: "GKE master-authorized network CIDR", Other: "GKE master-authorized network CIDRs"}},
		{Attribute: attrClusterApiServerOpenToInternet, Label: discovery_kit_api.PluralLabel{One: "GKE API server open to internet", Other: "GKE API server open to internet"}},
		{Attribute: attrClusterNetwork, Label: discovery_kit_api.PluralLabel{One: "GKE cluster network", Other: "GKE cluster networks"}},
		{Attribute: attrClusterSubnetwork, Label: discovery_kit_api.PluralLabel{One: "GKE cluster subnetwork", Other: "GKE cluster subnetworks"}},
		{Attribute: attrClusterWorkloadIdentityEnabled, Label: discovery_kit_api.PluralLabel{One: "GKE Workload Identity", Other: "GKE Workload Identity"}},
		{Attribute: attrClusterShieldedNodesEnabled, Label: discovery_kit_api.PluralLabel{One: "GKE Shielded Nodes", Other: "GKE Shielded Nodes"}},
		{Attribute: attrClusterBinaryAuthEvalMode, Label: discovery_kit_api.PluralLabel{One: "GKE Binary Authorization mode", Other: "GKE Binary Authorization modes"}},
		{Attribute: attrClusterLoggingService, Label: discovery_kit_api.PluralLabel{One: "GKE logging service", Other: "GKE logging services"}},
		{Attribute: attrClusterMonitoringService, Label: discovery_kit_api.PluralLabel{One: "GKE monitoring service", Other: "GKE monitoring services"}},
		{Attribute: "gcp.gke.cluster.node-locations", Label: discovery_kit_api.PluralLabel{One: "GKE node location", Other: "GKE node locations"}},
		{Attribute: attrProjectID, Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
		{Attribute: attrK8sClusterName, Label: discovery_kit_api.PluralLabel{One: "Kubernetes cluster name", Other: "Kubernetes cluster names"}},
	}
}

func (d *clusterDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := container.NewClusterManagerClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create GKE client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllClusters(ctx, client, access.ProjectID)
	}, ctx, "gke-cluster")
}

func getAllClusters(ctx context.Context, client clusterManagerApi, projectID string) ([]discovery_kit_api.Target, error) {
	// Wildcard location '-' returns clusters across all regions and zones in the project.
	parent := fmt.Sprintf("projects/%s/locations/-", projectID)
	resp, err := client.ListClusters(ctx, &containerpb.ListClustersRequest{Parent: parent})
	if err != nil {
		log.Warn().Err(err).Str("project", projectID).Msg("Failed to list GKE clusters")
		return nil, err
	}
	targets := make([]discovery_kit_api.Target, 0, len(resp.Clusters))
	for _, c := range resp.Clusters {
		targets = append(targets, toClusterTarget(c, projectID))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesGkeCluster), nil
}

type clusterManagerApi interface {
	ListClusters(ctx context.Context, req *containerpb.ListClustersRequest, opts ...gax.CallOption) (*containerpb.ListClustersResponse, error)
	ListNodePools(ctx context.Context, req *containerpb.ListNodePoolsRequest, opts ...gax.CallOption) (*containerpb.ListNodePoolsResponse, error)
}

func toClusterTarget(c *containerpb.Cluster, projectID string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes[attrProjectID] = []string{projectID}
	attributes[attrClusterName] = []string{c.Name}
	attributes[attrClusterLocation] = []string{c.Location}
	attributes[attrClusterLocationType] = []string{classifyLocation(c.Location)}

	// k8s.cluster-name = GKE cluster name (1:1 within a project; the extension-kubernetes discovery uses
	// the kubeconfig context, which for GKE is the cluster name unless explicitly overridden).
	attributes[attrK8sClusterName] = []string{c.Name}

	if c.CurrentMasterVersion != "" {
		attributes[attrClusterKubernetesVersion] = []string{c.CurrentMasterVersion}
	}
	if c.Status != containerpb.Cluster_STATUS_UNSPECIFIED {
		attributes["gcp.gke.cluster.status"] = []string{c.Status.String()}
	}
	if c.ReleaseChannel != nil && c.ReleaseChannel.Channel != containerpb.ReleaseChannel_UNSPECIFIED {
		attributes[attrClusterReleaseChannel] = []string{c.ReleaseChannel.Channel.String()}
	}
	if c.LoggingService != "" {
		attributes[attrClusterLoggingService] = []string{c.LoggingService}
	}
	if c.MonitoringService != "" {
		attributes[attrClusterMonitoringService] = []string{c.MonitoringService}
	}
	if c.Network != "" {
		attributes[attrClusterNetwork] = []string{c.Network}
	}
	if c.Subnetwork != "" {
		attributes[attrClusterSubnetwork] = []string{c.Subnetwork}
	}
	if len(c.Locations) > 0 {
		locs := append([]string(nil), c.Locations...)
		sort.Strings(locs)
		attributes["gcp.gke.cluster.node-locations"] = locs
	}

	// "private cluster" in GKE parlance = worker nodes have no public IPs (EnablePrivateNodes).
	// EnablePrivateEndpoint is a distinct control-plane setting (whether the API server has a public IP)
	// and drives api-server-open-to-internet below.
	privateNodes := c.PrivateClusterConfig != nil && c.PrivateClusterConfig.EnablePrivateNodes
	privateEndpoint := c.PrivateClusterConfig != nil && c.PrivateClusterConfig.EnablePrivateEndpoint
	attributes[attrClusterPrivateCluster] = []string{strconv.FormatBool(privateNodes)}

	manEnabled := false
	var manCidrs []string
	if c.MasterAuthorizedNetworksConfig != nil {
		manEnabled = c.MasterAuthorizedNetworksConfig.Enabled
		for _, b := range c.MasterAuthorizedNetworksConfig.CidrBlocks {
			if b != nil && b.CidrBlock != "" {
				manCidrs = append(manCidrs, b.CidrBlock)
			}
		}
	}
	attributes[attrClusterMasterAuthorizedNetsEnabled] = []string{strconv.FormatBool(manEnabled)}
	if len(manCidrs) > 0 {
		sort.Strings(manCidrs)
		attributes[attrClusterMasterAuthorizedNetsCidrs] = manCidrs
	}
	// True iff the API server is reachable from the public internet without IP restriction.
	// Private endpoint => not internet-reachable. Public endpoint AND no authorized-networks restriction => open.
	attributes[attrClusterApiServerOpenToInternet] = []string{strconv.FormatBool(!privateEndpoint && !manEnabled)}

	wiEnabled := c.WorkloadIdentityConfig != nil && c.WorkloadIdentityConfig.WorkloadPool != ""
	attributes[attrClusterWorkloadIdentityEnabled] = []string{strconv.FormatBool(wiEnabled)}

	shielded := c.ShieldedNodes != nil && c.ShieldedNodes.Enabled
	attributes[attrClusterShieldedNodesEnabled] = []string{strconv.FormatBool(shielded)}

	if c.BinaryAuthorization != nil && c.BinaryAuthorization.EvaluationMode != containerpb.BinaryAuthorization_EVALUATION_MODE_UNSPECIFIED {
		attributes[attrClusterBinaryAuthEvalMode] = []string{c.BinaryAuthorization.EvaluationMode.String()}
	}

	for k, v := range c.ResourceLabels {
		attributes[fmt.Sprintf("gcp.gke.cluster.label.%s", strings.ToLower(k))] = []string{v}
	}

	return discovery_kit_api.Target{
		Id:         fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, c.Location, c.Name),
		TargetType: TargetIDCluster,
		Label:      c.Name,
		Attributes: attributes,
	}
}

// classifyLocation returns "regional" if the location is a region (e.g. us-central1) or "zonal" if it's
// a zone (e.g. us-central1-a). GKE clusters use the same location field for both — regional clusters span
// multiple zones in the named region.
func classifyLocation(location string) string {
	// Zones have the form <region>-<letter>, e.g. us-central1-a. Regions have no trailing -letter.
	if i := strings.LastIndex(location, "-"); i >= 0 && len(location)-i == 2 {
		return "zonal"
	}
	return "regional"
}

func (d *clusterDiscovery) DescribeEnrichmentRules() []discovery_kit_api.TargetEnrichmentRule {
	rules := make([]discovery_kit_api.TargetEnrichmentRule, 0, len(gkeEnrichmentTargetTypes))
	for _, t := range gkeEnrichmentTargetTypes {
		rules = append(rules, gkeClusterToK8sEnrichmentRule(t))
	}
	return rules
}

func gkeClusterToK8sEnrichmentRule(destTargetType string) discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      fmt.Sprintf("com.steadybit.extension_gcp.gke.cluster-to-%s", destTargetType),
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDCluster,
			Selector: map[string]string{
				attrK8sClusterName: "${dest.k8s.cluster-name}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: destTargetType,
			Selector: map[string]string{
				attrK8sClusterName: "${src.k8s.cluster-name}",
			},
		},
		Attributes: gkeEnrichmentAttributes,
	}
}
