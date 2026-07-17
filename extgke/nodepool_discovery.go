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
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-gcp/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type nodePoolDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*nodePoolDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*nodePoolDiscovery)(nil)
)

func NewNodePoolDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&nodePoolDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *nodePoolDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDNodePool,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *nodePoolDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDNodePool,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "GKE node pool", Other: "GKE node pools"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "gcp.gke.cluster.name"},
				{Attribute: "gcp.gke.nodepool.kubernetes-version"},
				{Attribute: "gcp.gke.nodepool.machine-type"},
				{Attribute: "gcp.gke.nodepool.autoscaling.enabled"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *nodePoolDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.gke.cluster.name", Label: discovery_kit_api.PluralLabel{One: "GKE cluster name", Other: "GKE cluster names"}},
		{Attribute: "gcp.gke.cluster.location", Label: discovery_kit_api.PluralLabel{One: "GKE cluster location", Other: "GKE cluster locations"}},
		{Attribute: "gcp.gke.nodepool.name", Label: discovery_kit_api.PluralLabel{One: "GKE node pool name", Other: "GKE node pool names"}},
		{Attribute: "gcp.gke.nodepool.kubernetes-version", Label: discovery_kit_api.PluralLabel{One: "GKE node pool Kubernetes version", Other: "GKE node pool Kubernetes versions"}},
		{Attribute: "gcp.gke.nodepool.status", Label: discovery_kit_api.PluralLabel{One: "GKE node pool status", Other: "GKE node pool statuses"}},
		{Attribute: "gcp.gke.nodepool.machine-type", Label: discovery_kit_api.PluralLabel{One: "GKE node pool machine type", Other: "GKE node pool machine types"}},
		{Attribute: "gcp.gke.nodepool.disk-type", Label: discovery_kit_api.PluralLabel{One: "GKE node pool disk type", Other: "GKE node pool disk types"}},
		{Attribute: "gcp.gke.nodepool.disk-size-gb", Label: discovery_kit_api.PluralLabel{One: "GKE node pool disk size (GiB)", Other: "GKE node pool disk sizes (GiB)"}},
		{Attribute: "gcp.gke.nodepool.image-type", Label: discovery_kit_api.PluralLabel{One: "GKE node pool image type", Other: "GKE node pool image types"}},
		{Attribute: "gcp.gke.nodepool.preemptible", Label: discovery_kit_api.PluralLabel{One: "GKE node pool preemptible", Other: "GKE node pool preemptible"}},
		{Attribute: "gcp.gke.nodepool.spot", Label: discovery_kit_api.PluralLabel{One: "GKE node pool spot", Other: "GKE node pool spot"}},
		{Attribute: "gcp.gke.nodepool.autoscaling.enabled", Label: discovery_kit_api.PluralLabel{One: "GKE node pool autoscaling", Other: "GKE node pool autoscaling"}},
		{Attribute: "gcp.gke.nodepool.autoscaling.min-node-count", Label: discovery_kit_api.PluralLabel{One: "GKE node pool min node count", Other: "GKE node pool min node counts"}},
		{Attribute: "gcp.gke.nodepool.autoscaling.max-node-count", Label: discovery_kit_api.PluralLabel{One: "GKE node pool max node count", Other: "GKE node pool max node counts"}},
		{Attribute: "gcp.gke.nodepool.locations", Label: discovery_kit_api.PluralLabel{One: "GKE node pool location", Other: "GKE node pool locations"}},
		{Attribute: "gcp.gke.nodepool.max-pods-per-node", Label: discovery_kit_api.PluralLabel{One: "GKE node pool max pods per node", Other: "GKE node pool max pods per nodes"}},
		{Attribute: "gcp.gke.nodepool.management.auto-upgrade", Label: discovery_kit_api.PluralLabel{One: "GKE node pool auto-upgrade", Other: "GKE node pool auto-upgrade"}},
		{Attribute: "gcp.gke.nodepool.management.auto-repair", Label: discovery_kit_api.PluralLabel{One: "GKE node pool auto-repair", Other: "GKE node pool auto-repair"}},
		{Attribute: "gcp.gke.nodepool.upgrade-settings.max-surge", Label: discovery_kit_api.PluralLabel{One: "GKE node pool max surge", Other: "GKE node pool max surges"}},
		{Attribute: "gcp.gke.nodepool.upgrade-settings.max-unavailable", Label: discovery_kit_api.PluralLabel{One: "GKE node pool max unavailable", Other: "GKE node pool max unavailable"}},
		{Attribute: "gcp.gke.nodepool.instance-group-urls", Label: discovery_kit_api.PluralLabel{One: "GKE node pool MIG URL", Other: "GKE node pool MIG URLs"}},
		{Attribute: "gcp.project.id", Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
		{Attribute: "k8s.cluster-name", Label: discovery_kit_api.PluralLabel{One: "Kubernetes cluster name", Other: "Kubernetes cluster names"}},
	}
}

func (d *nodePoolDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := container.NewClusterManagerClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create GKE client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllNodePools(ctx, client, access.ProjectID)
	}, ctx, "gke-nodepool")
}

func getAllNodePools(ctx context.Context, client clusterManagerApi, projectID string) ([]discovery_kit_api.Target, error) {
	// First list clusters across all locations, then list node pools per cluster.
	resp, err := client.ListClusters(ctx, &containerpb.ListClustersRequest{
		Parent: fmt.Sprintf("projects/%s/locations/-", projectID),
	})
	if err != nil {
		return nil, err
	}
	targets := make([]discovery_kit_api.Target, 0)
	for _, c := range resp.Clusters {
		parent := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, c.Location, c.Name)
		nps, err := client.ListNodePools(ctx, &containerpb.ListNodePoolsRequest{Parent: parent})
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Str("cluster", c.Name).Msg("Failed to list GKE node pools")
			continue
		}
		for _, np := range nps.NodePools {
			targets = append(targets, toNodePoolTarget(np, c, projectID))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesGkeNodePool), nil
}

func toNodePoolTarget(np *containerpb.NodePool, cluster *containerpb.Cluster, projectID string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes["gcp.project.id"] = []string{projectID}
	attributes["gcp.gke.cluster.name"] = []string{cluster.Name}
	attributes["gcp.gke.cluster.location"] = []string{cluster.Location}
	attributes["k8s.cluster-name"] = []string{cluster.Name}
	attributes["gcp.gke.nodepool.name"] = []string{np.Name}

	if np.Version != "" {
		attributes["gcp.gke.nodepool.kubernetes-version"] = []string{np.Version}
	}
	if np.Status != containerpb.NodePool_STATUS_UNSPECIFIED {
		attributes["gcp.gke.nodepool.status"] = []string{np.Status.String()}
	}
	if np.Config != nil {
		if np.Config.MachineType != "" {
			attributes["gcp.gke.nodepool.machine-type"] = []string{np.Config.MachineType}
		}
		if np.Config.DiskType != "" {
			attributes["gcp.gke.nodepool.disk-type"] = []string{np.Config.DiskType}
		}
		if np.Config.DiskSizeGb > 0 {
			attributes["gcp.gke.nodepool.disk-size-gb"] = []string{strconv.Itoa(int(np.Config.DiskSizeGb))}
		}
		if np.Config.ImageType != "" {
			attributes["gcp.gke.nodepool.image-type"] = []string{np.Config.ImageType}
		}
		attributes["gcp.gke.nodepool.preemptible"] = []string{strconv.FormatBool(np.Config.Preemptible)}
		attributes["gcp.gke.nodepool.spot"] = []string{strconv.FormatBool(np.Config.Spot)}
	}
	asEnabled := np.Autoscaling != nil && np.Autoscaling.Enabled
	attributes["gcp.gke.nodepool.autoscaling.enabled"] = []string{strconv.FormatBool(asEnabled)}
	if asEnabled {
		attributes["gcp.gke.nodepool.autoscaling.min-node-count"] = []string{strconv.Itoa(int(np.Autoscaling.MinNodeCount))}
		attributes["gcp.gke.nodepool.autoscaling.max-node-count"] = []string{strconv.Itoa(int(np.Autoscaling.MaxNodeCount))}
	}
	if np.MaxPodsConstraint != nil && np.MaxPodsConstraint.MaxPodsPerNode > 0 {
		attributes["gcp.gke.nodepool.max-pods-per-node"] = []string{strconv.Itoa(int(np.MaxPodsConstraint.MaxPodsPerNode))}
	}
	if np.Management != nil {
		attributes["gcp.gke.nodepool.management.auto-upgrade"] = []string{strconv.FormatBool(np.Management.AutoUpgrade)}
		attributes["gcp.gke.nodepool.management.auto-repair"] = []string{strconv.FormatBool(np.Management.AutoRepair)}
	}
	if np.UpgradeSettings != nil {
		attributes["gcp.gke.nodepool.upgrade-settings.max-surge"] = []string{strconv.Itoa(int(np.UpgradeSettings.MaxSurge))}
		attributes["gcp.gke.nodepool.upgrade-settings.max-unavailable"] = []string{strconv.Itoa(int(np.UpgradeSettings.MaxUnavailable))}
	}
	if len(np.Locations) > 0 {
		locs := append([]string(nil), np.Locations...)
		sort.Strings(locs)
		attributes["gcp.gke.nodepool.locations"] = locs
	}
	if len(np.InstanceGroupUrls) > 0 {
		urls := append([]string(nil), np.InstanceGroupUrls...)
		sort.Strings(urls)
		attributes["gcp.gke.nodepool.instance-group-urls"] = urls
	}

	if np.Config != nil {
		for k, v := range np.Config.Labels {
			attributes[fmt.Sprintf("gcp.gke.nodepool.label.%s", strings.ToLower(k))] = []string{v}
		}
	}

	return discovery_kit_api.Target{
		Id:         fmt.Sprintf("projects/%s/locations/%s/clusters/%s/nodePools/%s", projectID, cluster.Location, cluster.Name, np.Name),
		TargetType: TargetIDNodePool,
		Label:      fmt.Sprintf("%s/%s", cluster.Name, np.Name),
		Attributes: attributes,
	}
}
