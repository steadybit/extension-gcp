/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmig

import (
	"context"
	"fmt"
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

type migDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*migDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*migDiscovery)(nil)
)

func NewMigDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&migDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *migDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDMig,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *migDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDMig,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Managed Instance Group", Other: "Managed Instance Groups"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "gcp.mig.scope"},
				{Attribute: "gcp.mig.target-size"},
				{Attribute: "gcp.mig.location"},
				{Attribute: "gcp.project.id"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *migDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.mig.name", Label: discovery_kit_api.PluralLabel{One: "MIG name", Other: "MIG names"}},
		{Attribute: "gcp.mig.scope", Label: discovery_kit_api.PluralLabel{One: "MIG scope", Other: "MIG scopes"}},
		{Attribute: "gcp.mig.location", Label: discovery_kit_api.PluralLabel{One: "MIG location", Other: "MIG locations"}},
		{Attribute: "gcp.mig.target-size", Label: discovery_kit_api.PluralLabel{One: "MIG target size", Other: "MIG target sizes"}},
		{Attribute: "gcp.mig.base-instance-name", Label: discovery_kit_api.PluralLabel{One: "MIG base instance name", Other: "MIG base instance names"}},
		{Attribute: "gcp.mig.instance-template", Label: discovery_kit_api.PluralLabel{One: "MIG instance template", Other: "MIG instance templates"}},
		{Attribute: "gcp.mig.distribution-policy.target-shape", Label: discovery_kit_api.PluralLabel{One: "MIG distribution shape", Other: "MIG distribution shapes"}},
		{Attribute: "gcp.mig.distribution-policy.zones", Label: discovery_kit_api.PluralLabel{One: "MIG distribution zone", Other: "MIG distribution zones"}},
		{Attribute: "gcp.mig.auto-healing-policies.health-check", Label: discovery_kit_api.PluralLabel{One: "MIG auto-healing health check", Other: "MIG auto-healing health checks"}},
		{Attribute: "gcp.mig.update-policy.type", Label: discovery_kit_api.PluralLabel{One: "MIG update policy type", Other: "MIG update policy types"}},
		{Attribute: "gcp.mig.update-policy.replacement-method", Label: discovery_kit_api.PluralLabel{One: "MIG update replacement method", Other: "MIG update replacement methods"}},
		{Attribute: "gcp.mig.update-policy.minimal-action", Label: discovery_kit_api.PluralLabel{One: "MIG update minimal action", Other: "MIG update minimal actions"}},
		{Attribute: "gcp.mig.stateful-policy.configured", Label: discovery_kit_api.PluralLabel{One: "MIG stateful policy configured", Other: "MIG stateful policy configured"}},
		{Attribute: "gcp.project.id", Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *migDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := compute.NewInstanceGroupManagersRESTClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create MIG client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllMigs(ctx, client, access.ProjectID)
	}, ctx, "mig")
}

// getAllMigs walks the aggregated list of MIGs across all zones and regions of the project.
func getAllMigs(ctx context.Context, client *compute.InstanceGroupManagersClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	it := client.AggregatedList(ctx, &computepb.AggregatedListInstanceGroupManagersRequest{Project: projectID})
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to aggregate-list MIGs")
			return nil, err
		}
		if pair.Value == nil {
			continue
		}
		scope, location := parseScope(pair.Key)
		for _, mig := range pair.Value.InstanceGroupManagers {
			targets = append(targets, toMigTarget(mig, scope, location, projectID))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesMig), nil
}

// parseScope turns the aggregated-list map key (e.g. "zones/us-central1-a" or "regions/europe-west1") into
// (scope, location) — scope is "zonal" or "regional", location is the zone or region name.
func parseScope(key string) (scope string, location string) {
	switch {
	case strings.HasPrefix(key, "zones/"):
		return "zonal", strings.TrimPrefix(key, "zones/")
	case strings.HasPrefix(key, "regions/"):
		return "regional", strings.TrimPrefix(key, "regions/")
	default:
		return "unknown", key
	}
}

func toMigTarget(mig *computepb.InstanceGroupManager, scope, location, projectID string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes["gcp.project.id"] = []string{projectID}
	attributes["gcp.mig.name"] = []string{mig.GetName()}
	attributes["gcp.mig.scope"] = []string{scope}
	attributes["gcp.mig.location"] = []string{location}
	attributes["gcp.mig.target-size"] = []string{strconv.Itoa(int(mig.GetTargetSize()))}
	if v := mig.GetBaseInstanceName(); v != "" {
		attributes["gcp.mig.base-instance-name"] = []string{v}
	}
	if v := mig.GetInstanceTemplate(); v != "" {
		attributes["gcp.mig.instance-template"] = []string{v}
	}

	if dp := mig.GetDistributionPolicy(); dp != nil {
		if v := dp.GetTargetShape(); v != "" {
			attributes["gcp.mig.distribution-policy.target-shape"] = []string{v}
		}
		zones := make([]string, 0, len(dp.GetZones()))
		for _, z := range dp.GetZones() {
			if z != nil && z.Zone != nil && *z.Zone != "" {
				zones = append(zones, *z.Zone)
			}
		}
		if len(zones) > 0 {
			attributes["gcp.mig.distribution-policy.zones"] = zones
		}
	}

	if up := mig.GetUpdatePolicy(); up != nil {
		if v := up.GetType(); v != "" {
			attributes["gcp.mig.update-policy.type"] = []string{v}
		}
		if v := up.GetReplacementMethod(); v != "" {
			attributes["gcp.mig.update-policy.replacement-method"] = []string{v}
		}
		if v := up.GetMinimalAction(); v != "" {
			attributes["gcp.mig.update-policy.minimal-action"] = []string{v}
		}
	}

	if ahps := mig.GetAutoHealingPolicies(); len(ahps) > 0 {
		hcs := make([]string, 0, len(ahps))
		for _, p := range ahps {
			if p != nil && p.HealthCheck != nil && *p.HealthCheck != "" {
				hcs = append(hcs, *p.HealthCheck)
			}
		}
		if len(hcs) > 0 {
			attributes["gcp.mig.auto-healing-policies.health-check"] = hcs
		}
	}

	attributes["gcp.mig.stateful-policy.configured"] = []string{strconv.FormatBool(mig.GetStatefulPolicy() != nil)}

	return discovery_kit_api.Target{
		Id:         mig.GetSelfLink(),
		TargetType: TargetIDMig,
		Label:      mig.GetName(),
		Attributes: attributes,
	}
}
