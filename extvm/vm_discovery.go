/*
* Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extvm

import (
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"errors"
	"fmt"
	"github.com/googleapis/gax-go/v2"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/iterator"
	"slices"
	"strings"
	"time"
)

type vmDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber          = (*vmDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber       = (*vmDiscovery)(nil)
	_ discovery_kit_sdk.EnrichmentRulesDescriber = (*vmDiscovery)(nil)
)

func NewVirtualMachineDiscovery() discovery_kit_sdk.TargetDiscovery {
	discovery := &vmDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 30*time.Second),
	)
}

func (d *vmDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDVM,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func (d *vmDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:      TargetIDVM,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),

		// Labels used in the UI
		Label: discovery_kit_api.PluralLabel{One: "Google Cloud Virtual Machine", Other: "Google Cloud Virtual Machines"},

		// Category for the targets to appear in
		Category: extutil.Ptr("cloud"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "gcp.zone"},
				{Attribute: "gcp.project.id"},
				{Attribute: "gcp-vm.status"},
				{Attribute: "gcp-kubernetes-engine.cluster.name"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "steadybit.label",
					Direction: "ASC",
				},
			},
		},
	}
}

func (d *vmDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "gcp-vm.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "VM name",
				Other: "VM names",
			},
		},
		{
			Attribute: "gcp-vm.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "VM ID",
				Other: "VM IDs",
			},
		},
		{
			Attribute: "gcp-vm.hostname",
			Label: discovery_kit_api.PluralLabel{
				One:   "Host name",
				Other: "Host names",
			},
		},
		{
			Attribute: "gcp-vm.description",
			Label: discovery_kit_api.PluralLabel{
				One:   "Description",
				Other: "Descriptions",
			},
		},
		{
			Attribute: "gcp-vm.cpu-platform",
			Label: discovery_kit_api.PluralLabel{
				One:   "CPU platform",
				Other: "CPU platforms",
			},
		},
		{
			Attribute: "gcp-vm.machine-type",
			Label: discovery_kit_api.PluralLabel{
				One:   "Machine type",
				Other: "Machine types",
			},
		},
		{
			Attribute: "gcp-vm.source-machine-image",
			Label: discovery_kit_api.PluralLabel{
				One:   "Source machine image",
				Other: "Source machine images",
			},
		},
		{
			Attribute: "gcp-vm.status",
			Label: discovery_kit_api.PluralLabel{
				One:   "Status",
				Other: "Statuses",
			},
		},
		{
			Attribute: "gcp-vm.status-message",
			Label: discovery_kit_api.PluralLabel{
				One:   "Status message",
				Other: "Status messages",
			},
		},
		{
			Attribute: "gcp.zone-url",
			Label: discovery_kit_api.PluralLabel{
				One:   "Zone URL",
				Other: "Zone URLs",
			},
		},
		{
			Attribute: "gcp-vm.tag",
			Label: discovery_kit_api.PluralLabel{
				One:   "Tags",
				Other: "Tags",
			},
		},
		{
			Attribute: "gcp-vm.label",
			Label: discovery_kit_api.PluralLabel{
				One:   "Label",
				Other: "Labels",
			},
		},

		{
			Attribute: "gcp.zone",
			Label: discovery_kit_api.PluralLabel{
				One:   "Zone",
				Other: "Zones",
			},
		},

		{
			Attribute: "gcp.project.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "Project ID",
				Other: "Project IDs",
			},
		},
		{
			Attribute: "gcp-kubernetes-engine.cluster.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "Cluster Name",
				Other: "Cluster Names",
			},
		},
		{
			Attribute: "gcp-kubernetes-engine.cluster.location",
			Label: discovery_kit_api.PluralLabel{
				One:   "Cluster Location",
				Other: "Cluster Locations",
			},
		},
	}
}

func (d *vmDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	instancesClient, err := getGcpInstancesClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	defer func() { _ = instancesClient.Close() }()

	instances, err := getAllVirtualMachinesInstances(ctx, instancesClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get all virtual machines: %w", err)
	}
	return instancesToTargets(instances), nil
}

type GCPInstancesApi interface {
	AggregatedList(ctx context.Context, req *computepb.AggregatedListInstancesRequest, opts ...gax.CallOption) *compute.InstancesScopedListPairIterator
}

func getAllVirtualMachinesInstances(ctx context.Context, client GCPInstancesApi) ([]*computepb.Instance, error) {
	projectID := config.Config.ProjectID
	if projectID == "" {
		log.Error().Msgf("project id is not set")
		return nil, errors.New("project id is not set")
	}
	req := &computepb.AggregatedListInstancesRequest{
		Project: projectID,
	}
	it := client.AggregatedList(ctx, req)
	allInstances := make([]*computepb.Instance, 0)
	for {
		pair, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Error().Msgf("failed to iterate through instances: %v", err)
			return nil, err
		}
		instances := pair.Value.Instances
		if len(instances) > 0 {
			log.Debug().Msgf("Instances for %s", pair.Key)
			for _, instance := range instances {
				log.Debug().Msgf("- %s %s\n", instance.GetName(), instance.GetMachineType())

				allInstances = append(allInstances, instance)

			}
		}
	}
	return allInstances, nil
}
func instancesToTargets(instances []*computepb.Instance) []discovery_kit_api.Target {
	targets := make([]discovery_kit_api.Target, 0)
	for _, instance := range instances {
		targets = instanceToTarget(instance, targets)
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesVM)
}

func instanceToTarget(instance *computepb.Instance, targets []discovery_kit_api.Target) []discovery_kit_api.Target {
	attributes := make(map[string][]string)

	attributes["gcp-vm.name"] = []string{getStringValue(instance.Name)}
	id := fmt.Sprintf("%d", *instance.Id)
	attributes["gcp-vm.id"] = []string{id}
	attributes["gcp-vm.hostname"] = []string{getHostname(instance)}
	attributes["gcp-vm.description"] = []string{getStringValue(instance.Description)}
	attributes["gcp-vm.cpu-platform"] = []string{getStringValue(instance.CpuPlatform)}
	attributes["gcp-vm.machine-type"] = []string{getStringValue(instance.MachineType)}
	attributes["gcp-vm.source-machine-image"] = []string{getStringValue(instance.SourceMachineImage)}
	attributes["gcp-vm.status"] = []string{getStringValue(instance.Status)}
	attributes["gcp-vm.status-message"] = []string{getStringValue(instance.StatusMessage)}
	attributes["gcp.zone-url"] = []string{getStringValue(instance.Zone)}
	attributes["gcp.zone"] = []string{getZone(instance)}
	attributes["gcp.project.id"] = []string{config.Config.ProjectID}
	attributes["gcp-kubernetes-engine.cluster.name"] = []string{getMetadata(instance.Metadata, "cluster-name")}
	attributes["gcp-kubernetes-engine.cluster.location"] = []string{getMetadata(instance.Metadata, "cluster-location")}

	for k, v := range instance.Labels {
		attributes[fmt.Sprintf("gcp-vm.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}
	if instance.Tags != nil {
		attributes["gcp-vm.tags"] = []string{strings.Join(instance.Tags.Items, ",")}
	}

	targets = append(targets, discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDVM,
		Label:      getStringValue(instance.Name),
		Attributes: attributes,
	})
	return targets
}

func getZone(instance *computepb.Instance) string {
	url := getStringValue(instance.Zone)
	lastIndex := strings.LastIndex(url, "/")
	if lastIndex > 0 {
		return url[lastIndex+1:]
	}
	return ""
}

func getHostname(instance *computepb.Instance) string {
	if instance.Hostname != nil {
		return *instance.Hostname
	}
	return getStringValue(instance.Name)
}

func getMetadata(metadata *computepb.Metadata, key string) string {
	if metadata != nil {
		for _, item := range metadata.Items {
			if getStringValue(item.Key) == key {
				return getStringValue(item.Value)
			}
		}
	}
	return ""
}

func getStringValue(val *string) string {
	if val != nil {
		return *val
	}
	return ""
}

func (d *vmDiscovery) DescribeEnrichmentRules() []discovery_kit_api.TargetEnrichmentRule {
	defaultEnrichmentTargetTypes := []string{
		"com.steadybit.extension_host.host",
		"com.steadybit.extension_kubernetes.kubernetes-node",
		"com.steadybit.extension_host_windows.host",
		"com.steadybit.extension_container.container",
	}
	rules := []discovery_kit_api.TargetEnrichmentRule{
		getToHostEnrichmentRule("host", "com.steadybit.extension_host.host"),
		getToHostEnrichmentRule("k8s-node", "com.steadybit.extension_kubernetes.kubernetes-node"),
		getToHostWindowsEnrichmentRule(),
		getToContainerEnrichmentRule(),
	}
	for _, targetType := range config.Config.EnrichVMDataForTargetTypes {
		if slices.Contains(defaultEnrichmentTargetTypes, targetType) {
			log.Warn().Msgf("Target type %s is already covered by default rules. Omitting.", targetType)
		} else {
			rules = append(rules, getVMToXEnrichmentRule(targetType))
		}
	}
	return rules
}

func getToHostEnrichmentRule(targetName string, targetType string) discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_gcp.gcp-vm-to-" + targetName,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"gcp-vm.hostname": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: targetType,
			Selector: map[string]string{
				"host.hostname": "${src.gcp-vm.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.label.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-kubernetes-engine.",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.zone",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.project.id",
			},
		},
	}
}

func getToHostWindowsEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_gcp.gcp-vm-to-host-windows",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"gcp-vm.id": "${dest.gcp-vm.id}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_host_windows.host",
			Selector: map[string]string{
				"gcp-vm.id": "${src.gcp-vm.id}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.label.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-kubernetes-engine.",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.zone",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.project.id",
			},
		},
	}
}

func getToContainerEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_gcp.gcp-vm-to-container",
		Version: extbuild.GetSemverVersionStringOrUnknown(),

		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"gcp-vm.hostname": "${dest.container.host}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_container.container",
			Selector: map[string]string{
				"container.host": "${src.gcp-vm.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.label.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-kubernetes-engine.",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.zone",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.project.id",
			},
		},
	}
}

func getVMToXEnrichmentRule(destTargetType string) discovery_kit_api.TargetEnrichmentRule {
	id := fmt.Sprintf("com.steadybit.extension_gcp.vm.instance-to-%s", destTargetType)
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      id,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"gcp-vm.hostname": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: destTargetType,
			Selector: map[string]string{
				"host.hostname": "${src.gcp-vm.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.zone",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.project.id",
			},
		},
	}
}
