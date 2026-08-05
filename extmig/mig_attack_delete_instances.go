/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmig

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-gcp/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/iterator"
)

// MigDeleteInstancesState holds the sampled instance URLs to recreate on Start.
// This attack calls the MIG's RecreateInstances API — the target VMs are deleted and replaced from the
// current instance template. Unlike DeleteInstances, RecreateInstances preserves the MIG's targetSize,
// so an autoscaler-free MIG (like a fixed-size stateful pool) doesn't stay undersized after the attack.
// The VMs get new hardware identity (new instance IDs, boot times) but keep their names and any
// stateful-policy state.
type MigDeleteInstancesState struct {
	ProjectID  string
	Scope      string // "zonal" or "regional"
	Location   string // zone (e.g. us-central1-a) or region (e.g. us-central1)
	MigName    string
	Percentage int
	Instances  []string
}

type migDeleteInstancesAttack struct {
	zonalClientProvider    func(ctx context.Context, projectID string) (zonalMigApi, func(), error)
	regionalClientProvider func(ctx context.Context, projectID string) (regionalMigApi, func(), error)
	rng                    func(n int) []int
}

type zonalMigApi interface {
	ListManagedInstances(ctx context.Context, req *computepb.ListManagedInstancesInstanceGroupManagersRequest, opts ...gaxOpt) *compute.ManagedInstanceIterator
	RecreateInstances(ctx context.Context, req *computepb.RecreateInstancesInstanceGroupManagerRequest, opts ...gaxOpt) (*compute.Operation, error)
}

type regionalMigApi interface {
	ListManagedInstances(ctx context.Context, req *computepb.ListManagedInstancesRegionInstanceGroupManagersRequest, opts ...gaxOpt) *compute.ManagedInstanceIterator
	RecreateInstances(ctx context.Context, req *computepb.RecreateInstancesRegionInstanceGroupManagerRequest, opts ...gaxOpt) (*compute.Operation, error)
}

// gaxOpt is a local alias for gax.CallOption keeping the interfaces above terse.
type gaxOpt = gax.CallOption

var _ action_kit_sdk.Action[MigDeleteInstancesState] = (*migDeleteInstancesAttack)(nil)

func NewMigDeleteInstancesAction() action_kit_sdk.Action[MigDeleteInstancesState] {
	return &migDeleteInstancesAttack{
		zonalClientProvider: func(ctx context.Context, projectID string) (zonalMigApi, func(), error) {
			access, err := utils.GetGcpAccess(projectID)
			if err != nil {
				return nil, nil, err
			}
			c, err := compute.NewInstanceGroupManagersRESTClient(ctx, access.ClientOptions...)
			if err != nil {
				return nil, nil, err
			}
			return c, func() { _ = c.Close() }, nil
		},
		regionalClientProvider: func(ctx context.Context, projectID string) (regionalMigApi, func(), error) {
			access, err := utils.GetGcpAccess(projectID)
			if err != nil {
				return nil, nil, err
			}
			c, err := compute.NewRegionInstanceGroupManagersRESTClient(ctx, access.ClientOptions...)
			if err != nil {
				return nil, nil, err
			}
			return c, func() { _ = c.Close() }, nil
		},
		rng: rand.Perm,
	}
}

func (a *migDeleteInstancesAttack) NewEmptyState() MigDeleteInstancesState {
	return MigDeleteInstancesState{}
}

func (a *migDeleteInstancesAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    MigDeleteInstancesActionId,
		Label: "Recreate MIG instances",
		Description: "Recreates a percentage of RUNNING instances in a Managed Instance Group. The MIG's targetSize is preserved (unlike deleteInstances, which shrinks it). Not reversible.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDMig,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by MIG name",
					Description: extutil.Ptr("Find MIG by name"),
					Query:       "gcp.mig.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("GCP"),
		Category:    extutil.Ptr("Compute Engine"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percentage",
				Label:        "Percentage of instances to recreate",
				Description:  extutil.Ptr("Percentage (1-100) of MIG's RUNNING instances to recreate. Defaults to 33%."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: extutil.Ptr("33"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
				MinValue:     extutil.Ptr(1),
				MaxValue:     extutil.Ptr(100),
			},
			{
				Name:         "confirmHighImpact",
				Label:        "Allow percentages above 50%",
				Description:  extutil.Ptr("Required to enable percentages above 50%. Acknowledges that more than half the MIG will be recreated simultaneously."),
				Type:         action_kit_api.ActionParameterTypeBoolean,
				DefaultValue: extutil.Ptr("false"),
				Order:        extutil.Ptr(2),
				Required:     extutil.Ptr(false),
			},
		},
	}
}

func (a *migDeleteInstancesAttack) Prepare(ctx context.Context, state *MigDeleteInstancesState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.ProjectID = mustHave(request.Target.Attributes, "gcp.project.id")
	state.Scope = mustHave(request.Target.Attributes, "gcp.mig.scope")
	state.Location = mustHave(request.Target.Attributes, "gcp.mig.location")
	state.MigName = mustHave(request.Target.Attributes, "gcp.mig.name")
	if state.ProjectID == "" || state.Scope == "" || state.Location == "" || state.MigName == "" {
		return nil, extension_kit.ToError("Target is missing one of: gcp.project.id, gcp.mig.scope, gcp.mig.location, gcp.mig.name", nil)
	}
	pct := extutil.ToInt(request.Config["percentage"])
	if pct < 1 || pct > 100 {
		return nil, extension_kit.ToError("percentage must be between 1 and 100.", nil)
	}
	confirmHigh := extutil.ToBool(request.Config["confirmHighImpact"])
	if pct > 50 && !confirmHigh {
		return nil, extension_kit.ToError("Percentages above 50% require the 'Allow percentages above 50%' flag — half the MIG will be recreated at once.", nil)
	}
	state.Percentage = pct

	allInstances, err := a.listRunningInstances(ctx, state)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to list MIG instances for %s/%s", state.Location, state.MigName), err)
	}
	if len(allInstances) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("MIG %s/%s has no RUNNING instances to recreate", state.Location, state.MigName), nil)
	}
	sort.Strings(allInstances)
	// Use math.Floor (not math.Ceil) so the sample never exceeds the requested
	// percentage. math.Ceil on e.g. 3 * 50 / 100 = 1.5 rounds up to 2 = 67 %,
	// which silently bypasses the confirmHighImpact gate at >50 %. Floor with a
	// >=1 clamp keeps the "always kill at least one" property.
	sampleSize := int(math.Floor(float64(len(allInstances)) * float64(pct) / 100.0))
	if sampleSize < 1 {
		sampleSize = 1
	}
	if sampleSize > len(allInstances) {
		sampleSize = len(allInstances)
	}
	// Second guard: on very small MIGs the >=1 clamp can push the effective
	// ratio above 50 % (e.g. pct=50 on 1 instance → 100 %). Refuse when this
	// happens unless confirmHighImpact was explicitly set — otherwise the
	// safety gate is silently bypassed by MIG size, not by user request.
	if sampleSize*2 > len(allInstances) && !confirmHigh {
		return nil, extension_kit.ToError(fmt.Sprintf(
			"Effective impact %d of %d instance(s) exceeds 50%% (small MIG rounds up to a full instance). Set 'Allow percentages above 50%%' to acknowledge.",
			sampleSize, len(allInstances)), nil)
	}
	perm := a.rng(len(allInstances))
	state.Instances = make([]string, 0, sampleSize)
	for i := 0; i < sampleSize; i++ {
		state.Instances = append(state.Instances, allInstances[perm[i]])
	}
	sort.Strings(state.Instances)
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Selected %d of %d RUNNING instance(s) (%d%%) in MIG %s/%s for recreation", sampleSize, len(allInstances), pct, state.Location, state.MigName),
		}}),
	}, nil
}

func (a *migDeleteInstancesAttack) listRunningInstances(ctx context.Context, state *MigDeleteInstancesState) ([]string, error) {
	result := make([]string, 0)
	switch state.Scope {
	case "zonal":
		client, closer, err := a.zonalClientProvider(ctx, state.ProjectID)
		if err != nil {
			return nil, err
		}
		defer closer()
		it := client.ListManagedInstances(ctx, &computepb.ListManagedInstancesInstanceGroupManagersRequest{
			Project:              state.ProjectID,
			Zone:                 state.Location,
			InstanceGroupManager: state.MigName,
		})
		for {
			mi, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, err
			}
			if mi.GetInstanceStatus() == "RUNNING" && mi.GetInstance() != "" {
				result = append(result, mi.GetInstance())
			}
		}
	case "regional":
		client, closer, err := a.regionalClientProvider(ctx, state.ProjectID)
		if err != nil {
			return nil, err
		}
		defer closer()
		it := client.ListManagedInstances(ctx, &computepb.ListManagedInstancesRegionInstanceGroupManagersRequest{
			Project:              state.ProjectID,
			Region:               state.Location,
			InstanceGroupManager: state.MigName,
		})
		for {
			mi, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, err
			}
			if mi.GetInstanceStatus() == "RUNNING" && mi.GetInstance() != "" {
				result = append(result, mi.GetInstance())
			}
		}
	default:
		return nil, fmt.Errorf("unsupported MIG scope %q", state.Scope)
	}
	return result, nil
}

func (a *migDeleteInstancesAttack) Start(ctx context.Context, state *MigDeleteInstancesState) (*action_kit_api.StartResult, error) {
	if len(state.Instances) == 0 {
		return nil, extension_kit.ToError("No instances selected for recreation.", nil)
	}
	switch state.Scope {
	case "zonal":
		client, closer, err := a.zonalClientProvider(ctx, state.ProjectID)
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to create MIG client for project %s", state.ProjectID), err)
		}
		defer closer()
		_, err = client.RecreateInstances(ctx, &computepb.RecreateInstancesInstanceGroupManagerRequest{
			Project:              state.ProjectID,
			Zone:                 state.Location,
			InstanceGroupManager: state.MigName,
			InstanceGroupManagersRecreateInstancesRequestResource: &computepb.InstanceGroupManagersRecreateInstancesRequest{
				Instances: state.Instances,
			},
		})
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to recreate instances in MIG %s/%s", state.Location, state.MigName), err)
		}
	case "regional":
		client, closer, err := a.regionalClientProvider(ctx, state.ProjectID)
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to create regional MIG client for project %s", state.ProjectID), err)
		}
		defer closer()
		_, err = client.RecreateInstances(ctx, &computepb.RecreateInstancesRegionInstanceGroupManagerRequest{
			Project:              state.ProjectID,
			Region:               state.Location,
			InstanceGroupManager: state.MigName,
			RegionInstanceGroupManagersRecreateRequestResource: &computepb.RegionInstanceGroupManagersRecreateRequest{
				Instances: state.Instances,
			},
		})
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to recreate instances in regional MIG %s/%s", state.Location, state.MigName), err)
		}
	default:
		return nil, extension_kit.ToError(fmt.Sprintf("unsupported MIG scope %q", state.Scope), nil)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Recreation requested for %d instance(s) in MIG %s/%s. The MIG's targetSize is preserved.", len(state.Instances), state.Location, state.MigName),
		}}),
	}, nil
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
