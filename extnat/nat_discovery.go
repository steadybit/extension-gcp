/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extnat

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-gcp/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/iterator"
)

type natDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*natDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*natDiscovery)(nil)
)

func NewNatDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&natDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *natDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDCloudNat,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *natDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDCloudNat,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Cloud NAT", Other: "Cloud NATs"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: attrRegion},
				{Attribute: attrSourceSubnetworkIpRanges},
				{Attribute: attrSubnetworkCount},
				{Attribute: attrProjectID},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *natDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.cloud-nat.name", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT name", Other: "Cloud NAT names"}},
		{Attribute: "gcp.cloud-nat.router", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT router", Other: "Cloud NAT routers"}},
		{Attribute: "gcp.cloud-nat.network", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT network", Other: "Cloud NAT networks"}},
		{Attribute: attrRegion, Label: discovery_kit_api.PluralLabel{One: "Cloud NAT region", Other: "Cloud NAT regions"}},
		{Attribute: attrSourceSubnetworkIpRanges, Label: discovery_kit_api.PluralLabel{One: "Cloud NAT source subnetwork IP ranges mode", Other: "Cloud NAT source subnetwork IP ranges modes"}},
		{Attribute: "gcp.cloud-nat.nat-ip-allocate-option", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT IP allocation option", Other: "Cloud NAT IP allocation options"}},
		{Attribute: "gcp.cloud-nat.nat-ips", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT IP", Other: "Cloud NAT IPs"}},
		{Attribute: "gcp.cloud-nat.endpoint-types", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT endpoint type", Other: "Cloud NAT endpoint types"}},
		{Attribute: "gcp.cloud-nat.subnetworks", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT subnetwork", Other: "Cloud NAT subnetworks"}},
		{Attribute: attrSubnetworkCount, Label: discovery_kit_api.PluralLabel{One: "Cloud NAT subnetwork count", Other: "Cloud NAT subnetwork counts"}},
		{Attribute: "gcp.cloud-nat.min-ports-per-vm", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT min ports per VM", Other: "Cloud NAT min ports per VM"}},
		{Attribute: "gcp.cloud-nat.log-config.enable", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT logging", Other: "Cloud NAT logging"}},
		{Attribute: "gcp.cloud-nat.enable-dynamic-port-allocation", Label: discovery_kit_api.PluralLabel{One: "Cloud NAT dynamic port allocation", Other: "Cloud NAT dynamic port allocation"}},
		{Attribute: attrProjectID, Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *natDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := compute.NewRoutersRESTClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Routers client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllNats(ctx, client, access.ProjectID)
	}, ctx, "cloud-nat")
}

func getAllNats(ctx context.Context, client *compute.RoutersClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	it := client.AggregatedList(ctx, &computepb.AggregatedListRoutersRequest{Project: projectID})
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to aggregate-list routers")
			return nil, err
		}
		if pair.Value == nil {
			continue
		}
		region := strings.TrimPrefix(pair.Key, "regions/")
		for _, router := range pair.Value.Routers {
			for _, nat := range router.GetNats() {
				targets = append(targets, toNatTarget(router, nat, region, projectID))
			}
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesCloudNat), nil
}

func toNatTarget(router *computepb.Router, nat *computepb.RouterNat, region, projectID string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes[attrProjectID] = []string{projectID}
	attributes["gcp.cloud-nat.name"] = []string{nat.GetName()}
	attributes["gcp.cloud-nat.router"] = []string{router.GetName()}
	attributes[attrRegion] = []string{region}
	if v := router.GetNetwork(); v != "" {
		attributes["gcp.cloud-nat.network"] = []string{v}
	}
	if v := nat.GetSourceSubnetworkIpRangesToNat(); v != "" {
		attributes[attrSourceSubnetworkIpRanges] = []string{v}
	}
	if v := nat.GetNatIpAllocateOption(); v != "" {
		attributes["gcp.cloud-nat.nat-ip-allocate-option"] = []string{v}
	}
	if ips := nat.GetNatIps(); len(ips) > 0 {
		sorted := append([]string(nil), ips...)
		sort.Strings(sorted)
		attributes["gcp.cloud-nat.nat-ips"] = sorted
	}
	if et := nat.GetEndpointTypes(); len(et) > 0 {
		sorted := append([]string(nil), et...)
		sort.Strings(sorted)
		attributes["gcp.cloud-nat.endpoint-types"] = sorted
	}
	subnets := nat.GetSubnetworks()
	subnetNames := make([]string, 0, len(subnets))
	for _, s := range subnets {
		if s != nil && s.Name != nil && *s.Name != "" {
			subnetNames = append(subnetNames, *s.Name)
		}
	}
	sort.Strings(subnetNames)
	if len(subnetNames) > 0 {
		attributes["gcp.cloud-nat.subnetworks"] = subnetNames
	}
	attributes[attrSubnetworkCount] = []string{strconv.Itoa(len(subnets))}
	if v := nat.GetMinPortsPerVm(); v != 0 {
		attributes["gcp.cloud-nat.min-ports-per-vm"] = []string{strconv.Itoa(int(v))}
	}
	if nat.LogConfig != nil {
		attributes["gcp.cloud-nat.log-config.enable"] = []string{strconv.FormatBool(nat.LogConfig.GetEnable())}
	}
	if nat.EnableDynamicPortAllocation != nil {
		attributes["gcp.cloud-nat.enable-dynamic-port-allocation"] = []string{strconv.FormatBool(*nat.EnableDynamicPortAllocation)}
	}

	return discovery_kit_api.Target{
		Id:         fmt.Sprintf("%s/nats/%s", router.GetSelfLink(), nat.GetName()),
		TargetType: TargetIDCloudNat,
		Label:      fmt.Sprintf("%s/%s", router.GetName(), nat.GetName()),
		Attributes: attributes,
	}
}
