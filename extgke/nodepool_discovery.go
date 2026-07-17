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
				{Attribute: attrClusterName},
				{Attribute: attrNodePoolKubernetesVersion},
				{Attribute: attrNodePoolMachineType},
				{Attribute: attrNodePoolAutoscalingEnabled},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *nodePoolDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: attrClusterName, Label: discovery_kit_api.PluralLabel{One: "GKE cluster name", Other: "GKE cluster names"}},
		{Attribute: "gcp.gke.cluster.location", Label: discovery_kit_api.PluralLabel{One: "GKE cluster location", Other: "GKE cluster locations"}},
		{Attribute: "gcp.gke.nodepool.name", Label: discovery_kit_api.PluralLabel{One: "GKE node pool name", Other: "GKE node pool names"}},
		{Attribute: attrNodePoolKubernetesVersion, Label: discovery_kit_api.PluralLabel{One: "GKE node pool Kubernetes version", Other: "GKE node pool Kubernetes versions"}},
		{Attribute: "gcp.gke.nodepool.status", Label: discovery_kit_api.PluralLabel{One: "GKE node pool status", Other: "GKE node pool statuses"}},
		{Attribute: attrNodePoolMachineType, Label: discovery_kit_api.PluralLabel{One: "GKE node pool machine type", Other: "GKE node pool machine types"}},
		{Attribute: "gcp.gke.nodepool.disk-type", Label: discovery_kit_api.PluralLabel{One: "GKE node pool disk type", Other: "GKE node pool disk types"}},
		{Attribute: "gcp.gke.nodepool.disk-size-gb", Label: discovery_kit_api.PluralLabel{One: "GKE node pool disk size (GiB)", Other: "GKE node pool disk sizes (GiB)"}},
		{Attribute: "gcp.gke.nodepool.image-type", Label: discovery_kit_api.PluralLabel{One: "GKE node pool image type", Other: "GKE node pool image types"}},
		{Attribute: "gcp.gke.nodepool.preemptible", Label: discovery_kit_api.PluralLabel{One: "GKE node pool preemptible", Other: "GKE node pool preemptible"}},
		{Attribute: "gcp.gke.nodepool.spot", Label: discovery_kit_api.PluralLabel{One: "GKE node pool spot", Other: "GKE node pool spot"}},
		{Attribute: attrNodePoolAutoscalingEnabled, Label: discovery_kit_api.PluralLabel{One: "GKE node pool autoscaling", Other: "GKE node pool autoscaling"}},
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
	attributes[attrClusterName] = []string{cluster.Name}
	attributes["gcp.gke.cluster.location"] = []string{cluster.Location}
	attributes["k8s.cluster-name"] = []string{cluster.Name}
	attributes["gcp.gke.nodepool.name"] = []string{np.Name}

	utils.SetStr(attributes, attrNodePoolKubernetesVersion, np.Version)

	addNodePoolStatusAttrs(attributes, np.Status)
	addNodePoolConfigAttrs(attributes, np.Config)
	addNodePoolAutoscalingAttrs(attributes, np.Autoscaling)
	addNodePoolMaxPodsAttrs(attributes, np.MaxPodsConstraint)
	addNodePoolManagementAttrs(attributes, np.Management)
	addNodePoolUpgradeSettingsAttrs(attributes, np.UpgradeSettings)
	addNodePoolLocationsAttrs(attributes, np.Locations)
	addNodePoolInstanceGroupUrlsAttrs(attributes, np.InstanceGroupUrls)

	return discovery_kit_api.Target{
		Id:         fmt.Sprintf("projects/%s/locations/%s/clusters/%s/nodePools/%s", projectID, cluster.Location, cluster.Name, np.Name),
		TargetType: TargetIDNodePool,
		Label:      fmt.Sprintf("%s/%s", cluster.Name, np.Name),
		Attributes: attributes,
	}
}

func addNodePoolStatusAttrs(attrs map[string][]string, status containerpb.NodePool_Status) {
	if status == containerpb.NodePool_STATUS_UNSPECIFIED {
		return
	}
	attrs["gcp.gke.nodepool.status"] = []string{status.String()}
}

func addNodePoolConfigAttrs(attrs map[string][]string, cfg *containerpb.NodeConfig) {
	if cfg == nil {
		return
	}
	utils.SetStr(attrs, attrNodePoolMachineType, cfg.MachineType)
	utils.SetStr(attrs, "gcp.gke.nodepool.disk-type", cfg.DiskType)
	utils.SetInt64IfPositive(attrs, "gcp.gke.nodepool.disk-size-gb", int64(cfg.DiskSizeGb))
	utils.SetStr(attrs, "gcp.gke.nodepool.image-type", cfg.ImageType)
	utils.SetBool(attrs, "gcp.gke.nodepool.preemptible", cfg.Preemptible)
	utils.SetBool(attrs, "gcp.gke.nodepool.spot", cfg.Spot)
	for k, v := range cfg.Labels {
		utils.SetStr(attrs, fmt.Sprintf("gcp.gke.nodepool.label.%s", strings.ToLower(k)), v)
	}
}

func addNodePoolAutoscalingAttrs(attrs map[string][]string, as *containerpb.NodePoolAutoscaling) {
	asEnabled := as != nil && as.Enabled
	utils.SetBool(attrs, attrNodePoolAutoscalingEnabled, asEnabled)
	if !asEnabled {
		return
	}
	attrs["gcp.gke.nodepool.autoscaling.min-node-count"] = []string{strconv.Itoa(int(as.MinNodeCount))}
	attrs["gcp.gke.nodepool.autoscaling.max-node-count"] = []string{strconv.Itoa(int(as.MaxNodeCount))}
}

func addNodePoolMaxPodsAttrs(attrs map[string][]string, mpc *containerpb.MaxPodsConstraint) {
	if mpc == nil {
		return
	}
	utils.SetInt64IfPositive(attrs, "gcp.gke.nodepool.max-pods-per-node", mpc.MaxPodsPerNode)
}

func addNodePoolManagementAttrs(attrs map[string][]string, m *containerpb.NodeManagement) {
	if m == nil {
		return
	}
	utils.SetBool(attrs, "gcp.gke.nodepool.management.auto-upgrade", m.AutoUpgrade)
	utils.SetBool(attrs, "gcp.gke.nodepool.management.auto-repair", m.AutoRepair)
}

func addNodePoolUpgradeSettingsAttrs(attrs map[string][]string, us *containerpb.NodePool_UpgradeSettings) {
	if us == nil {
		return
	}
	attrs["gcp.gke.nodepool.upgrade-settings.max-surge"] = []string{strconv.Itoa(int(us.MaxSurge))}
	attrs["gcp.gke.nodepool.upgrade-settings.max-unavailable"] = []string{strconv.Itoa(int(us.MaxUnavailable))}
}

func addNodePoolLocationsAttrs(attrs map[string][]string, locations []string) {
	if len(locations) == 0 {
		return
	}
	locs := append([]string(nil), locations...)
	sort.Strings(locs)
	attrs["gcp.gke.nodepool.locations"] = locs
}

func addNodePoolInstanceGroupUrlsAttrs(attrs map[string][]string, urls []string) {
	if len(urls) == 0 {
		return
	}
	out := append([]string(nil), urls...)
	sort.Strings(out)
	attrs["gcp.gke.nodepool.instance-group-urls"] = out
}
