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

// MigDeleteInstancesState holds the sampled instance URLs to delete on Start.
// This attack is destructive and is NOT reversible: deleted instances are gone; the MIG creates new
// replacements per its scaling/heal policies. Recovery time depends on the MIG's autoscaler / surge
// configuration; a MIG with autoscaling disabled and targetSize manually managed will stay undersized
// until an operator intervenes.
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
	DeleteInstances(ctx context.Context, req *computepb.DeleteInstancesInstanceGroupManagerRequest, opts ...gaxOpt) (*compute.Operation, error)
}

type regionalMigApi interface {
	ListManagedInstances(ctx context.Context, req *computepb.ListManagedInstancesRegionInstanceGroupManagersRequest, opts ...gaxOpt) *compute.ManagedInstanceIterator
	DeleteInstances(ctx context.Context, req *computepb.DeleteInstancesRegionInstanceGroupManagerRequest, opts ...gaxOpt) (*compute.Operation, error)
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
		Label: "Delete MIG instances",
		Description: "Destructively deletes a percentage of RUNNING instances from a Managed Instance Group. The MIG creates new replacements per " +
			"its scaling/heal policies — typical recovery is minutes, but a MIG without autoscaling can stay undersized indefinitely. " +
			"Validates that workloads on the MIG tolerate the loss of N% of nodes. " +
			"This attack is not reversible: the deleted instances are gone. Percentages above 50% require explicit confirmation.",
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
		Technology:  extutil.Ptr("Google Cloud"),
		Category:    extutil.Ptr("Compute Engine"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percentage",
				Label:        "Percentage of instances to delete",
				Description:  extutil.Ptr("Percentage (1-100) of MIG's RUNNING instances to delete. Defaults to 33%."),
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
				Description:  extutil.Ptr("Required to enable percentages above 50%. Acknowledges that more than half the MIG will be deleted simultaneously."),
				Type:         action_kit_api.ActionParameterTypeBoolean,
				DefaultValue: extutil.Ptr("false"),
				Order:        extutil.Ptr(2),
				Required:     extutil.Ptr(false),
			},
		},
	}
}

func (a *migDeleteInstancesAttack) Prepare(ctx context.Context, state *MigDeleteInstancesState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	if err := hydrateStateFromTarget(state, request.Target.Attributes); err != nil {
		return nil, err
	}
	pct, err := validatePercentage(request.Config)
	if err != nil {
		return nil, err
	}
	state.Percentage = pct

	allInstances, err := a.listRunningInstances(ctx, state)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to list MIG instances for %s/%s", state.Location, state.MigName), err)
	}
	if len(allInstances) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("MIG %s/%s has no RUNNING instances to delete", state.Location, state.MigName), nil)
	}
	sort.Strings(allInstances)
	state.Instances = sampleInstances(allInstances, pct, a.rng)
	sort.Strings(state.Instances)
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Selected %d of %d RUNNING instance(s) (%d%%) in MIG %s/%s for deletion", len(state.Instances), len(allInstances), pct, state.Location, state.MigName),
		}}),
	}, nil
}

func hydrateStateFromTarget(state *MigDeleteInstancesState, attrs map[string][]string) error {
	state.ProjectID = mustHave(attrs, "gcp.project.id")
	state.Scope = mustHave(attrs, "gcp.mig.scope")
	state.Location = mustHave(attrs, "gcp.mig.location")
	state.MigName = mustHave(attrs, "gcp.mig.name")
	if state.ProjectID == "" || state.Scope == "" || state.Location == "" || state.MigName == "" {
		return extension_kit.ToError("Target is missing one of: gcp.project.id, gcp.mig.scope, gcp.mig.location, gcp.mig.name", nil)
	}
	return nil
}

func validatePercentage(cfg map[string]interface{}) (int, error) {
	pct := extutil.ToInt(cfg["percentage"])
	if pct < 1 || pct > 100 {
		return 0, extension_kit.ToError("percentage must be between 1 and 100.", nil)
	}
	if pct > 50 && !extutil.ToBool(cfg["confirmHighImpact"]) {
		return 0, extension_kit.ToError("Percentages above 50% require the 'Allow percentages above 50%' flag — half the MIG will be deleted at once.", nil)
	}
	return pct, nil
}

func sampleInstances(all []string, pct int, rng func(n int) []int) []string {
	sampleSize := int(math.Ceil(float64(len(all)) * float64(pct) / 100.0))
	if sampleSize < 1 {
		sampleSize = 1
	}
	if sampleSize > len(all) {
		sampleSize = len(all)
	}
	perm := rng(len(all))
	result := make([]string, 0, sampleSize)
	for i := 0; i < sampleSize; i++ {
		result = append(result, all[perm[i]])
	}
	return result
}

func (a *migDeleteInstancesAttack) listRunningInstances(ctx context.Context, state *MigDeleteInstancesState) ([]string, error) {
	switch state.Scope {
	case "zonal":
		return a.listZonalRunningInstances(ctx, state)
	case "regional":
		return a.listRegionalRunningInstances(ctx, state)
	default:
		return nil, fmt.Errorf("unsupported MIG scope %q", state.Scope)
	}
}

func (a *migDeleteInstancesAttack) listZonalRunningInstances(ctx context.Context, state *MigDeleteInstancesState) ([]string, error) {
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
	return collectRunningInstances(it)
}

func (a *migDeleteInstancesAttack) listRegionalRunningInstances(ctx context.Context, state *MigDeleteInstancesState) ([]string, error) {
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
	return collectRunningInstances(it)
}

func collectRunningInstances(it *compute.ManagedInstanceIterator) ([]string, error) {
	result := make([]string, 0)
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
	return result, nil
}

func (a *migDeleteInstancesAttack) Start(ctx context.Context, state *MigDeleteInstancesState) (*action_kit_api.StartResult, error) {
	if len(state.Instances) == 0 {
		return nil, extension_kit.ToError("No instances selected for deletion.", nil)
	}
	switch state.Scope {
	case "zonal":
		client, closer, err := a.zonalClientProvider(ctx, state.ProjectID)
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to create MIG client for project %s", state.ProjectID), err)
		}
		defer closer()
		_, err = client.DeleteInstances(ctx, &computepb.DeleteInstancesInstanceGroupManagerRequest{
			Project:              state.ProjectID,
			Zone:                 state.Location,
			InstanceGroupManager: state.MigName,
			InstanceGroupManagersDeleteInstancesRequestResource: &computepb.InstanceGroupManagersDeleteInstancesRequest{
				Instances: state.Instances,
			},
		})
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to delete instances from MIG %s/%s", state.Location, state.MigName), err)
		}
	case "regional":
		client, closer, err := a.regionalClientProvider(ctx, state.ProjectID)
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to create regional MIG client for project %s", state.ProjectID), err)
		}
		defer closer()
		_, err = client.DeleteInstances(ctx, &computepb.DeleteInstancesRegionInstanceGroupManagerRequest{
			Project:              state.ProjectID,
			Region:               state.Location,
			InstanceGroupManager: state.MigName,
			RegionInstanceGroupManagersDeleteInstancesRequestResource: &computepb.RegionInstanceGroupManagersDeleteInstancesRequest{
				Instances: state.Instances,
			},
		})
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to delete instances from regional MIG %s/%s", state.Location, state.MigName), err)
		}
	default:
		return nil, extension_kit.ToError(fmt.Sprintf("unsupported MIG scope %q", state.Scope), nil)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Deletion requested for %d instance(s) in MIG %s/%s. The MIG will replace them.", len(state.Instances), state.Location, state.MigName),
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
