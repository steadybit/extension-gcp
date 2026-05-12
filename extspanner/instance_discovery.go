/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extspanner

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
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
		Label:    discovery_kit_api.PluralLabel{One: "Spanner instance", Other: "Spanner instances"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "gcp.spanner.instance.config"},
				{Attribute: "gcp.spanner.instance.edition"},
				{Attribute: "gcp.spanner.instance.processing-units"},
				{Attribute: "gcp.project.id"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *instanceDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.spanner.instance.name", Label: discovery_kit_api.PluralLabel{One: "Spanner instance name", Other: "Spanner instance names"}},
		{Attribute: "gcp.spanner.instance.display-name", Label: discovery_kit_api.PluralLabel{One: "Spanner instance display name", Other: "Spanner instance display names"}},
		{Attribute: "gcp.spanner.instance.config", Label: discovery_kit_api.PluralLabel{One: "Spanner instance config", Other: "Spanner instance configs"}},
		{Attribute: "gcp.spanner.instance.edition", Label: discovery_kit_api.PluralLabel{One: "Spanner instance edition", Other: "Spanner instance editions"}},
		{Attribute: "gcp.spanner.instance.type", Label: discovery_kit_api.PluralLabel{One: "Spanner instance type", Other: "Spanner instance types"}},
		{Attribute: "gcp.spanner.instance.state", Label: discovery_kit_api.PluralLabel{One: "Spanner instance state", Other: "Spanner instance states"}},
		{Attribute: "gcp.spanner.instance.node-count", Label: discovery_kit_api.PluralLabel{One: "Spanner instance node count", Other: "Spanner instance node counts"}},
		{Attribute: "gcp.spanner.instance.processing-units", Label: discovery_kit_api.PluralLabel{One: "Spanner instance processing units", Other: "Spanner instance processing units"}},
		{Attribute: "gcp.spanner.instance.autoscaling.configured", Label: discovery_kit_api.PluralLabel{One: "Spanner instance autoscaling configured", Other: "Spanner instance autoscaling configured"}},
		{Attribute: "gcp.spanner.instance.default-backup-schedule-type", Label: discovery_kit_api.PluralLabel{One: "Spanner default backup schedule", Other: "Spanner default backup schedules"}},
		{Attribute: "gcp.project.id", Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *instanceDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := instance.NewInstanceAdminClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Spanner instance admin client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllInstances(ctx, client, access.ProjectID)
	}, ctx, "spanner-instance")
}

func getAllInstances(ctx context.Context, client *instance.InstanceAdminClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	it := client.ListInstances(ctx, &instancepb.ListInstancesRequest{Parent: fmt.Sprintf("projects/%s", projectID)})
	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to list Spanner instances")
			return nil, err
		}
		targets = append(targets, toInstanceTarget(inst, projectID))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesSpanner), nil
}

func toInstanceTarget(inst *instancepb.Instance, projectID string) discovery_kit_api.Target {
	// inst.Name is "projects/<p>/instances/<name>"
	name := inst.Name
	if i := strings.LastIndex(inst.Name, "/"); i >= 0 {
		name = inst.Name[i+1:]
	}
	// inst.Config is "projects/<p>/instanceConfigs/<config>"
	configName := inst.Config
	if i := strings.LastIndex(inst.Config, "/"); i >= 0 {
		configName = inst.Config[i+1:]
	}

	attributes := make(map[string][]string)
	attributes["gcp.project.id"] = []string{projectID}
	attributes["gcp.spanner.instance.name"] = []string{name}
	if inst.DisplayName != "" {
		attributes["gcp.spanner.instance.display-name"] = []string{inst.DisplayName}
	}
	if configName != "" {
		attributes["gcp.spanner.instance.config"] = []string{configName}
	}
	if inst.Edition != instancepb.Instance_EDITION_UNSPECIFIED {
		attributes["gcp.spanner.instance.edition"] = []string{inst.Edition.String()}
	}
	if inst.InstanceType != instancepb.Instance_INSTANCE_TYPE_UNSPECIFIED {
		attributes["gcp.spanner.instance.type"] = []string{inst.InstanceType.String()}
	}
	if inst.State != instancepb.Instance_STATE_UNSPECIFIED {
		attributes["gcp.spanner.instance.state"] = []string{inst.State.String()}
	}
	if inst.NodeCount > 0 {
		attributes["gcp.spanner.instance.node-count"] = []string{strconv.Itoa(int(inst.NodeCount))}
	}
	if inst.ProcessingUnits > 0 {
		attributes["gcp.spanner.instance.processing-units"] = []string{strconv.Itoa(int(inst.ProcessingUnits))}
	}
	attributes["gcp.spanner.instance.autoscaling.configured"] = []string{strconv.FormatBool(inst.AutoscalingConfig != nil)}
	if inst.DefaultBackupScheduleType != instancepb.Instance_DEFAULT_BACKUP_SCHEDULE_TYPE_UNSPECIFIED {
		attributes["gcp.spanner.instance.default-backup-schedule-type"] = []string{inst.DefaultBackupScheduleType.String()}
	}

	for k, v := range inst.Labels {
		attributes[fmt.Sprintf("gcp.spanner.instance.label.%s", strings.ToLower(k))] = []string{v}
	}

	return discovery_kit_api.Target{
		Id:         inst.Name,
		TargetType: TargetIDInstance,
		Label:      name,
		Attributes: attributes,
	}
}
