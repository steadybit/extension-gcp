/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extcloudsql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-gcp/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/sqladmin/v1"
)

type instanceDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*instanceDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*instanceDiscovery)(nil)
)

func NewInstanceDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&instanceDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *instanceDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDInstance,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *instanceDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDInstance,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Cloud SQL instance", Other: "Cloud SQL instances"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: attrDatabaseVersion},
				{Attribute: attrTier},
				{Attribute: attrAvailabilityType},
				{Attribute: attrRegion},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *instanceDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.cloudsql.instance.name", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL instance name", Other: "Cloud SQL instance names"}},
		{Attribute: attrDatabaseVersion, Label: discovery_kit_api.PluralLabel{One: "Cloud SQL database version", Other: "Cloud SQL database versions"}},
		{Attribute: attrTier, Label: discovery_kit_api.PluralLabel{One: "Cloud SQL tier", Other: "Cloud SQL tiers"}},
		{Attribute: attrAvailabilityType, Label: discovery_kit_api.PluralLabel{One: "Cloud SQL availability type", Other: "Cloud SQL availability types"}},
		{Attribute: attrRegion, Label: discovery_kit_api.PluralLabel{One: "Cloud SQL region", Other: "Cloud SQL regions"}},
		{Attribute: "gcp.cloudsql.gce-zone", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL GCE zone", Other: "Cloud SQL GCE zones"}},
		{Attribute: "gcp.cloudsql.secondary-gce-zone", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL secondary zone", Other: "Cloud SQL secondary zones"}},
		{Attribute: "gcp.cloudsql.state", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL state", Other: "Cloud SQL states"}},
		{Attribute: "gcp.cloudsql.instance-type", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL instance type", Other: "Cloud SQL instance types"}},
		{Attribute: "gcp.cloudsql.backup-enabled", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL backups enabled", Other: "Cloud SQL backups enabled"}},
		{Attribute: "gcp.cloudsql.point-in-time-recovery-enabled", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL PITR enabled", Other: "Cloud SQL PITR enabled"}},
		{Attribute: "gcp.cloudsql.deletion-protection-enabled", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL deletion protection", Other: "Cloud SQL deletion protection"}},
		{Attribute: "gcp.cloudsql.public-network-access", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL public network access", Other: "Cloud SQL public network access"}},
		{Attribute: "gcp.cloudsql.private-network", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL private network", Other: "Cloud SQL private networks"}},
		{Attribute: "gcp.cloudsql.require-ssl", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL require SSL", Other: "Cloud SQL require SSL"}},
		{Attribute: "gcp.cloudsql.disk-type", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL disk type", Other: "Cloud SQL disk types"}},
		{Attribute: "gcp.cloudsql.disk-size-gb", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL disk size (GiB)", Other: "Cloud SQL disk sizes (GiB)"}},
		{Attribute: "gcp.cloudsql.maintenance-window.day", Label: discovery_kit_api.PluralLabel{One: "Cloud SQL maintenance day", Other: "Cloud SQL maintenance days"}},
		{Attribute: "gcp.project.id", Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *instanceDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		svc, err := sqladmin.NewService(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Cloud SQL client for project '%s': %w", access.ProjectID, err)
		}
		return getAllInstances(ctx, svc, access.ProjectID)
	}, ctx, "cloudsql-instance")
}

func getAllInstances(ctx context.Context, svc *sqladmin.Service, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	call := svc.Instances.List(projectID).Context(ctx)
	if err := call.Pages(ctx, func(resp *sqladmin.InstancesListResponse) error {
		for _, inst := range resp.Items {
			targets = append(targets, toInstanceTarget(inst, projectID))
		}
		return nil
	}); err != nil {
		log.Warn().Err(err).Str("project", projectID).Msg("Failed to list Cloud SQL instances")
		return nil, err
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesCloudSql), nil
}

func toInstanceTarget(inst *sqladmin.DatabaseInstance, projectID string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes["gcp.project.id"] = []string{projectID}
	attributes["gcp.cloudsql.instance.name"] = []string{inst.Name}
	utils.SetStr(attributes, attrDatabaseVersion, inst.DatabaseVersion)
	utils.SetStr(attributes, attrRegion, inst.Region)
	utils.SetStr(attributes, "gcp.cloudsql.gce-zone", inst.GceZone)
	utils.SetStr(attributes, "gcp.cloudsql.secondary-gce-zone", inst.SecondaryGceZone)
	utils.SetStr(attributes, "gcp.cloudsql.state", inst.State)
	utils.SetStr(attributes, "gcp.cloudsql.instance-type", inst.InstanceType)
	addSettingsAttrs(attributes, inst.Settings)

	return discovery_kit_api.Target{
		Id:         fmt.Sprintf("projects/%s/instances/%s", projectID, inst.Name),
		TargetType: TargetIDInstance,
		Label:      inst.Name,
		Attributes: attributes,
	}
}

func addSettingsAttrs(attrs map[string][]string, s *sqladmin.Settings) {
	if s == nil {
		return
	}
	utils.SetStr(attrs, attrTier, s.Tier)
	utils.SetStr(attrs, attrAvailabilityType, s.AvailabilityType)
	addBackupAttrs(attrs, s.BackupConfiguration)
	utils.SetBool(attrs, "gcp.cloudsql.deletion-protection-enabled", s.DeletionProtectionEnabled)
	addIpConfigAttrs(attrs, s.IpConfiguration)
	utils.SetStr(attrs, "gcp.cloudsql.disk-type", s.DataDiskType)
	utils.SetInt64IfPositive(attrs, "gcp.cloudsql.disk-size-gb", s.DataDiskSizeGb)
	addMaintWindowAttrs(attrs, s.MaintenanceWindow)
	for k, v := range s.UserLabels {
		utils.SetStr(attrs, fmt.Sprintf("gcp.cloudsql.label.%s", strings.ToLower(k)), v)
	}
}

func addBackupAttrs(attrs map[string][]string, b *sqladmin.BackupConfiguration) {
	if b == nil {
		return
	}
	utils.SetBool(attrs, "gcp.cloudsql.backup-enabled", b.Enabled)
	utils.SetBool(attrs, "gcp.cloudsql.point-in-time-recovery-enabled", b.PointInTimeRecoveryEnabled)
}

func addIpConfigAttrs(attrs map[string][]string, ip *sqladmin.IpConfiguration) {
	if ip == nil {
		return
	}
	utils.SetBool(attrs, "gcp.cloudsql.public-network-access", ip.Ipv4Enabled)
	utils.SetStr(attrs, "gcp.cloudsql.private-network", ip.PrivateNetwork)
	utils.SetBool(attrs, "gcp.cloudsql.require-ssl", ip.RequireSsl)
}

func addMaintWindowAttrs(attrs map[string][]string, m *sqladmin.MaintenanceWindow) {
	if m == nil {
		return
	}
	utils.SetInt64IfPositive(attrs, "gcp.cloudsql.maintenance-window.day", m.Day)
}
