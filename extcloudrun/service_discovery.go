/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extcloudrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
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

type serviceDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*serviceDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*serviceDiscovery)(nil)
)

func NewServiceDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&serviceDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *serviceDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDService,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *serviceDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDService,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Cloud Run service", Other: "Cloud Run services"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: attrLocation},
				{Attribute: attrIngress},
				{Attribute: attrProjectID},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *serviceDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.cloudrun.service.name", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service name", Other: "Cloud Run service names"}},
		{Attribute: attrLocation, Label: discovery_kit_api.PluralLabel{One: "Cloud Run service location", Other: "Cloud Run service locations"}},
		{Attribute: attrIngress, Label: discovery_kit_api.PluralLabel{One: "Cloud Run service ingress", Other: "Cloud Run service ingress"}},
		{Attribute: "gcp.cloudrun.service.launch-stage", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service launch stage", Other: "Cloud Run service launch stages"}},
		{Attribute: "gcp.cloudrun.service.invoker-iam-disabled", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service invoker IAM disabled", Other: "Cloud Run service invoker IAM disabled"}},
		{Attribute: "gcp.cloudrun.service.iap-enabled", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service IAP enabled", Other: "Cloud Run service IAP enabled"}},
		{Attribute: "gcp.cloudrun.service.scaling.min-instance-count", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service min instances", Other: "Cloud Run service min instances"}},
		{Attribute: "gcp.cloudrun.service.scaling.scaling-mode", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service scaling mode", Other: "Cloud Run service scaling modes"}},
		{Attribute: "gcp.cloudrun.service.template.max-instance-request-concurrency", Label: discovery_kit_api.PluralLabel{One: "Cloud Run max request concurrency", Other: "Cloud Run max request concurrencies"}},
		{Attribute: "gcp.cloudrun.service.template.timeout", Label: discovery_kit_api.PluralLabel{One: "Cloud Run request timeout", Other: "Cloud Run request timeouts"}},
		{Attribute: "gcp.cloudrun.service.template.service-account", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service account", Other: "Cloud Run service accounts"}},
		{Attribute: "gcp.cloudrun.service.template.scaling.min-instance-count", Label: discovery_kit_api.PluralLabel{One: "Cloud Run revision min instances", Other: "Cloud Run revision min instances"}},
		{Attribute: "gcp.cloudrun.service.template.scaling.max-instance-count", Label: discovery_kit_api.PluralLabel{One: "Cloud Run revision max instances", Other: "Cloud Run revision max instances"}},
		{Attribute: "gcp.cloudrun.service.urls", Label: discovery_kit_api.PluralLabel{One: "Cloud Run service URL", Other: "Cloud Run service URLs"}},
		{Attribute: attrProjectID, Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *serviceDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := run.NewServicesClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Cloud Run services client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllServices(ctx, client, access.ProjectID)
	}, ctx, "cloudrun-service")
}

func getAllServices(ctx context.Context, client *run.ServicesClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	// Cloud Run supports the `-` wildcard to list services across all locations in a project.
	it := client.ListServices(ctx, &runpb.ListServicesRequest{Parent: fmt.Sprintf("projects/%s/locations/-", projectID)})
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to list Cloud Run services")
			return nil, err
		}
		targets = append(targets, toServiceTarget(s, projectID))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesCloudRun), nil
}

func toServiceTarget(s *runpb.Service, projectID string) discovery_kit_api.Target {
	// s.Name is "projects/<p>/locations/<region>/services/<name>"
	location, name := parseServiceName(s.Name)

	attributes := make(map[string][]string)
	attributes[attrProjectID] = []string{projectID}
	attributes["gcp.cloudrun.service.name"] = []string{name}
	utils.SetStr(attributes, attrLocation, location)
	if s.Ingress != runpb.IngressTraffic_INGRESS_TRAFFIC_UNSPECIFIED {
		attributes[attrIngress] = []string{s.Ingress.String()}
	}
	if s.LaunchStage != 0 {
		attributes["gcp.cloudrun.service.launch-stage"] = []string{s.LaunchStage.String()}
	}
	utils.SetBool(attributes, "gcp.cloudrun.service.invoker-iam-disabled", s.InvokerIamDisabled)
	utils.SetBool(attributes, "gcp.cloudrun.service.iap-enabled", s.IapEnabled)

	addServiceScalingAttrs(attributes, s.Scaling)
	addServiceTemplateAttrs(attributes, s.Template)

	if len(s.Urls) > 0 {
		attributes["gcp.cloudrun.service.urls"] = append([]string(nil), s.Urls...)
	}

	for k, v := range s.Labels {
		attributes[fmt.Sprintf("gcp.cloudrun.service.label.%s", strings.ToLower(k))] = []string{v}
	}

	return discovery_kit_api.Target{
		Id:         s.Name,
		TargetType: TargetIDService,
		Label:      name,
		Attributes: attributes,
	}
}

func addServiceScalingAttrs(attrs map[string][]string, sc *runpb.ServiceScaling) {
	if sc == nil {
		return
	}
	utils.SetInt64IfPositive(attrs, "gcp.cloudrun.service.scaling.min-instance-count", int64(sc.MinInstanceCount))
	if sc.ScalingMode != runpb.ServiceScaling_SCALING_MODE_UNSPECIFIED {
		attrs["gcp.cloudrun.service.scaling.scaling-mode"] = []string{sc.ScalingMode.String()}
	}
}

func addServiceTemplateAttrs(attrs map[string][]string, tpl *runpb.RevisionTemplate) {
	if tpl == nil {
		return
	}
	utils.SetInt64IfPositive(attrs, "gcp.cloudrun.service.template.max-instance-request-concurrency", int64(tpl.MaxInstanceRequestConcurrency))
	if tpl.Timeout != nil {
		attrs["gcp.cloudrun.service.template.timeout"] = []string{tpl.Timeout.AsDuration().String()}
	}
	utils.SetStr(attrs, "gcp.cloudrun.service.template.service-account", tpl.ServiceAccount)
	addRevisionScalingAttrs(attrs, tpl.Scaling)
}

func addRevisionScalingAttrs(attrs map[string][]string, sc *runpb.RevisionScaling) {
	if sc == nil {
		return
	}
	utils.SetInt64IfPositive(attrs, "gcp.cloudrun.service.template.scaling.min-instance-count", int64(sc.MinInstanceCount))
	utils.SetInt64IfPositive(attrs, "gcp.cloudrun.service.template.scaling.max-instance-count", int64(sc.MaxInstanceCount))
}

func parseServiceName(full string) (location, name string) {
	// projects/<p>/locations/<region>/services/<name>
	parts := strings.Split(full, "/")
	for i := 0; i+1 < len(parts); i++ {
		switch parts[i] {
		case "locations":
			location = parts[i+1]
		case "services":
			name = parts[i+1]
		}
	}
	return location, name
}
