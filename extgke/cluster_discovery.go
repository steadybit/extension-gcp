/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
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
	{Matcher: discovery_kit_api.Equals, Name: "gcp.project.id"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.name"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.location"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.location-type"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.kubernetes-version"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.release-channel"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.private-cluster"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.master-authorized-networks-enabled"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.master-authorized-networks-cidrs"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.api-server-open-to-internet"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.network"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.subnetwork"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.workload-identity-enabled"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.shielded-nodes-enabled"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.binary-authorization-evaluation-mode"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.logging-service"},
	{Matcher: discovery_kit_api.Equals, Name: "gcp.gke.cluster.monitoring-service"},
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
				{Attribute: "gcp.gke.cluster.kubernetes-version"},
				{Attribute: "gcp.gke.cluster.location"},
				{Attribute: "gcp.project.id"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *clusterDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.gke.cluster.name", Label: discovery_kit_api.PluralLabel{One: "GKE cluster name", Other: "GKE cluster names"}},
		{Attribute: "gcp.gke.cluster.location", Label: discovery_kit_api.PluralLabel{One: "GKE cluster location", Other: "GKE cluster locations"}},
		{Attribute: "gcp.gke.cluster.location-type", Label: discovery_kit_api.PluralLabel{One: "GKE cluster location type", Other: "GKE cluster location types"}},
		{Attribute: "gcp.gke.cluster.kubernetes-version", Label: discovery_kit_api.PluralLabel{One: "GKE Kubernetes version", Other: "GKE Kubernetes versions"}},
		{Attribute: "gcp.gke.cluster.release-channel", Label: discovery_kit_api.PluralLabel{One: "GKE release channel", Other: "GKE release channels"}},
		{Attribute: "gcp.gke.cluster.status", Label: discovery_kit_api.PluralLabel{One: "GKE cluster status", Other: "GKE cluster statuses"}},
		{Attribute: "gcp.gke.cluster.private-cluster", Label: discovery_kit_api.PluralLabel{One: "GKE private cluster", Other: "GKE private clusters"}},
		{Attribute: "gcp.gke.cluster.master-authorized-networks-enabled", Label: discovery_kit_api.PluralLabel{One: "GKE master-authorized-networks", Other: "GKE master-authorized-networks"}},
		{Attribute: "gcp.gke.cluster.master-authorized-networks-cidrs", Label: discovery_kit_api.PluralLabel{One: "GKE master-authorized network CIDR", Other: "GKE master-authorized network CIDRs"}},
		{Attribute: "gcp.gke.cluster.api-server-open-to-internet", Label: discovery_kit_api.PluralLabel{One: "GKE API server open to internet", Other: "GKE API server open to internet"}},
		{Attribute: "gcp.gke.cluster.network", Label: discovery_kit_api.PluralLabel{One: "GKE cluster network", Other: "GKE cluster networks"}},
		{Attribute: "gcp.gke.cluster.subnetwork", Label: discovery_kit_api.PluralLabel{One: "GKE cluster subnetwork", Other: "GKE cluster subnetworks"}},
		{Attribute: "gcp.gke.cluster.workload-identity-enabled", Label: discovery_kit_api.PluralLabel{One: "GKE Workload Identity", Other: "GKE Workload Identity"}},
		{Attribute: "gcp.gke.cluster.shielded-nodes-enabled", Label: discovery_kit_api.PluralLabel{One: "GKE Shielded Nodes", Other: "GKE Shielded Nodes"}},
		{Attribute: "gcp.gke.cluster.binary-authorization-evaluation-mode", Label: discovery_kit_api.PluralLabel{One: "GKE Binary Authorization mode", Other: "GKE Binary Authorization modes"}},
		{Attribute: "gcp.gke.cluster.logging-service", Label: discovery_kit_api.PluralLabel{One: "GKE logging service", Other: "GKE logging services"}},
		{Attribute: "gcp.gke.cluster.monitoring-service", Label: discovery_kit_api.PluralLabel{One: "GKE monitoring service", Other: "GKE monitoring services"}},
		{Attribute: "gcp.gke.cluster.node-locations", Label: discovery_kit_api.PluralLabel{One: "GKE node location", Other: "GKE node locations"}},
		{Attribute: "gcp.project.id", Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
		{Attribute: "k8s.cluster-name", Label: discovery_kit_api.PluralLabel{One: "Kubernetes cluster name", Other: "Kubernetes cluster names"}},
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
	attributes["gcp.project.id"] = []string{projectID}
	attributes["gcp.gke.cluster.name"] = []string{c.Name}
	attributes["gcp.gke.cluster.location"] = []string{c.Location}
	attributes["gcp.gke.cluster.location-type"] = []string{classifyLocation(c.Location)}

	// k8s.cluster-name = GKE cluster name (1:1 within a project; the extension-kubernetes discovery uses
	// the kubeconfig context, which for GKE is the cluster name unless explicitly overridden).
	attributes["k8s.cluster-name"] = []string{c.Name}

	if c.CurrentMasterVersion != "" {
		attributes["gcp.gke.cluster.kubernetes-version"] = []string{c.CurrentMasterVersion}
	}
	if c.Status != containerpb.Cluster_STATUS_UNSPECIFIED {
		attributes["gcp.gke.cluster.status"] = []string{c.Status.String()}
	}
	if c.ReleaseChannel != nil && c.ReleaseChannel.Channel != containerpb.ReleaseChannel_UNSPECIFIED {
		attributes["gcp.gke.cluster.release-channel"] = []string{c.ReleaseChannel.Channel.String()}
	}
	if c.LoggingService != "" {
		attributes["gcp.gke.cluster.logging-service"] = []string{c.LoggingService}
	}
	if c.MonitoringService != "" {
		attributes["gcp.gke.cluster.monitoring-service"] = []string{c.MonitoringService}
	}
	if c.Network != "" {
		attributes["gcp.gke.cluster.network"] = []string{c.Network}
	}
	if c.Subnetwork != "" {
		attributes["gcp.gke.cluster.subnetwork"] = []string{c.Subnetwork}
	}
	if len(c.Locations) > 0 {
		locs := append([]string(nil), c.Locations...)
		sort.Strings(locs)
		attributes["gcp.gke.cluster.node-locations"] = locs
	}

	private := c.PrivateClusterConfig != nil && c.PrivateClusterConfig.EnablePrivateEndpoint
	attributes["gcp.gke.cluster.private-cluster"] = []string{strconv.FormatBool(private)}

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
	attributes["gcp.gke.cluster.master-authorized-networks-enabled"] = []string{strconv.FormatBool(manEnabled)}
	if len(manCidrs) > 0 {
		sort.Strings(manCidrs)
		attributes["gcp.gke.cluster.master-authorized-networks-cidrs"] = manCidrs
	}
	// True iff the API server is reachable from the public internet without IP restriction.
	// Private endpoint => not internet-reachable. Public endpoint AND no authorized-networks restriction => open.
	attributes["gcp.gke.cluster.api-server-open-to-internet"] = []string{strconv.FormatBool(!private && !manEnabled)}

	wiEnabled := c.WorkloadIdentityConfig != nil && c.WorkloadIdentityConfig.WorkloadPool != ""
	attributes["gcp.gke.cluster.workload-identity-enabled"] = []string{strconv.FormatBool(wiEnabled)}

	shielded := c.ShieldedNodes != nil && c.ShieldedNodes.Enabled
	attributes["gcp.gke.cluster.shielded-nodes-enabled"] = []string{strconv.FormatBool(shielded)}

	if c.BinaryAuthorization != nil && c.BinaryAuthorization.EvaluationMode != containerpb.BinaryAuthorization_EVALUATION_MODE_UNSPECIFIED {
		attributes["gcp.gke.cluster.binary-authorization-evaluation-mode"] = []string{c.BinaryAuthorization.EvaluationMode.String()}
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
				"k8s.cluster-name": "${dest.k8s.cluster-name}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: destTargetType,
			Selector: map[string]string{
				"k8s.cluster-name": "${src.k8s.cluster-name}",
			},
		},
		Attributes: gkeEnrichmentAttributes,
	}
}
