/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extgke

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-gcp/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/iterator"
)

// NodePoolTerminateInstancesState captures enough state to execute the terminate-instances attack.
// This attack is destructive and is NOT reversible: deleted instances are gone; the MIG behind the node
// pool creates new replacements driven by its scaling/heal policies (mirrors the EKS / AKS pattern).
// Replacement time depends on cluster-autoscaler and surge configuration; on a misconfigured pool the
// pool can remain undersized indefinitely.
type NodePoolTerminateInstancesState struct {
	ProjectID    string
	ClusterName  string
	NodePoolName string
	Location     string // GKE cluster location (region or zone)
	Percentage   int
	// InstancesByMig maps "<zone>/<migName>" → list of full instance URLs selected for deletion.
	InstancesByMig map[string][]string
}

type migInstancesApi interface {
	ListManagedInstances(ctx context.Context, req *computepb.ListManagedInstancesInstanceGroupManagersRequest, opts ...gax.CallOption) *compute.ManagedInstanceIterator
	DeleteInstances(ctx context.Context, req *computepb.DeleteInstancesInstanceGroupManagerRequest, opts ...gax.CallOption) (*compute.Operation, error)
}

type nodePoolTerminateInstancesAttack struct {
	migClientProvider func(ctx context.Context, projectID string) (migInstancesApi, func(), error)
	rng               func(n int) []int
}

var _ action_kit_sdk.Action[NodePoolTerminateInstancesState] = (*nodePoolTerminateInstancesAttack)(nil)

func NewNodePoolTerminateInstancesAction() action_kit_sdk.Action[NodePoolTerminateInstancesState] {
	return &nodePoolTerminateInstancesAttack{
		migClientProvider: func(ctx context.Context, projectID string) (migInstancesApi, func(), error) {
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
		rng: rand.Perm,
	}
}

func (a *nodePoolTerminateInstancesAttack) NewEmptyState() NodePoolTerminateInstancesState {
	return NodePoolTerminateInstancesState{}
}

func (a *nodePoolTerminateInstancesAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    NodePoolTerminateInstancesActionId,
		Label: "Terminate GKE node pool instances",
		Description: "Destructively deletes a percentage of instances from a GKE node pool via the underlying Managed Instance Group(s). " +
			"The MIG creates new replacements driven by its scaling/heal policies — typical recovery is minutes, but a node pool with " +
			"cluster-autoscaler disabled or surge=0 can stay undersized indefinitely. Validates pod rescheduling, PDB enforcement, " +
			"cluster-autoscaler scale-up, and stateful workload zonal failover. " +
			"This attack is not reversible: the deleted instances are gone. Percentages above 50% require explicit confirmation.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDNodePool,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by cluster name and node pool name",
					Description: extutil.Ptr("Find GKE node pool by cluster name and node pool name"),
					Query:       "gcp.gke.cluster.name=\"\" and gcp.gke.nodepool.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Google Cloud"),
		Category:    extutil.Ptr("GKE"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percentage",
				Label:        "Percentage of instances to terminate",
				Description:  extutil.Ptr("Percentage (1-100) of node pool's instances to terminate. Defaults to 33%."),
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
				Description:  extutil.Ptr("Required to enable percentages above 50%. Acknowledges that more than half the node pool will be deleted simultaneously."),
				Type:         action_kit_api.ActionParameterTypeBoolean,
				DefaultValue: extutil.Ptr("false"),
				Order:        extutil.Ptr(2),
				Required:     extutil.Ptr(false),
			},
		},
	}
}

func (a *nodePoolTerminateInstancesAttack) Prepare(ctx context.Context, state *NodePoolTerminateInstancesState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.ProjectID = mustHave(request.Target.Attributes, "gcp.project.id")
	state.ClusterName = mustHave(request.Target.Attributes, attrClusterName)
	state.NodePoolName = mustHave(request.Target.Attributes, "gcp.gke.nodepool.name")
	state.Location = mustHave(request.Target.Attributes, "gcp.gke.cluster.location")
	if state.ProjectID == "" || state.ClusterName == "" || state.NodePoolName == "" || state.Location == "" {
		return nil, extension_kit.ToError("Target is missing one of: gcp.project.id, gcp.gke.cluster.name, gcp.gke.nodepool.name, gcp.gke.cluster.location", nil)
	}
	pct := extutil.ToInt(request.Config["percentage"])
	if pct < 1 || pct > 100 {
		return nil, extension_kit.ToError("percentage must be between 1 and 100.", nil)
	}
	confirmHigh := extutil.ToBool(request.Config["confirmHighImpact"])
	if pct > 50 && !confirmHigh {
		return nil, extension_kit.ToError("Percentages above 50% require the 'Allow percentages above 50%' flag — half the node pool will be deleted at once.", nil)
	}
	state.Percentage = pct

	// Fetch fresh InstanceGroupUrls for the node pool — they may have changed since discovery.
	access, err := utils.GetGcpAccess(state.ProjectID)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to get GCP access for project %s", state.ProjectID), err)
	}
	gke, err := container.NewClusterManagerClient(ctx, access.ClientOptions...)
	if err != nil {
		return nil, extension_kit.ToError("Failed to create GKE client", err)
	}
	defer func() { _ = gke.Close() }()
	np, err := gke.GetNodePool(ctx, &containerpb.GetNodePoolRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/clusters/%s/nodePools/%s", state.ProjectID, state.Location, state.ClusterName, state.NodePoolName),
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe GKE node pool %s/%s", state.ClusterName, state.NodePoolName), err)
	}
	if len(np.InstanceGroupUrls) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("GKE node pool %s/%s has no underlying instance groups", state.ClusterName, state.NodePoolName), nil)
	}

	migClient, closer, err := a.migClientProvider(ctx, state.ProjectID)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to create MIG client for project %s", state.ProjectID), err)
	}
	defer closer()

	type instanceRef struct {
		migZone string
		migName string
		url     string
	}
	allInstances := make([]instanceRef, 0)
	for _, igURL := range np.InstanceGroupUrls {
		zone, name, ok := parseZonalMIGUrl(igURL)
		if !ok {
			// Regional MIGs aren't supported by GKE for node pools, but skip unknown URL shapes defensively.
			continue
		}
		it := migClient.ListManagedInstances(ctx, &computepb.ListManagedInstancesInstanceGroupManagersRequest{
			Project:              state.ProjectID,
			Zone:                 zone,
			InstanceGroupManager: name,
		})
		for {
			mi, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, extension_kit.ToError(fmt.Sprintf("Failed to list instances of MIG %s/%s", zone, name), err)
			}
			// Only target currently running instances; skip those already being created/deleted/repaired.
			if mi.GetInstanceStatus() != "RUNNING" {
				continue
			}
			if mi.GetInstance() == "" {
				continue
			}
			allInstances = append(allInstances, instanceRef{migZone: zone, migName: name, url: mi.GetInstance()})
		}
	}
	if len(allInstances) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("GKE node pool %s/%s has no RUNNING instances to terminate", state.ClusterName, state.NodePoolName), nil)
	}

	// Sort for determinism, then random-sample N%.
	sort.Slice(allInstances, func(i, j int) bool { return allInstances[i].url < allInstances[j].url })
	// Use math.Floor (not math.Ceil) so the sample never exceeds the requested
	// percentage — math.Ceil silently amplifies the effective impact past the
	// 50 % gate on small node pools.
	sampleSize := int(math.Floor(float64(len(allInstances)) * float64(pct) / 100.0))
	if sampleSize < 1 {
		sampleSize = 1
	}
	if sampleSize > len(allInstances) {
		sampleSize = len(allInstances)
	}
	// Small-pool guard: the >=1 clamp can still push the effective ratio above
	// 50 % (e.g. pct=50 on 1 node → 100 %). Refuse unless confirmHighImpact was
	// explicitly set.
	if sampleSize*2 > len(allInstances) && !confirmHigh {
		return nil, extension_kit.ToError(fmt.Sprintf(
			"Effective impact %d of %d node(s) exceeds 50%% (small node pool rounds up to a full node). Set 'Allow percentages above 50%%' to acknowledge.",
			sampleSize, len(allInstances)), nil)
	}
	perm := a.rng(len(allInstances))
	state.InstancesByMig = make(map[string][]string)
	for i := 0; i < sampleSize; i++ {
		ref := allInstances[perm[i]]
		key := fmt.Sprintf("%s/%s", ref.migZone, ref.migName)
		state.InstancesByMig[key] = append(state.InstancesByMig[key], ref.url)
	}
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Selected %d of %d RUNNING instance(s) (%d%%) in GKE node pool %s/%s for deletion across %d MIG(s)", sampleSize, len(allInstances), pct, state.ClusterName, state.NodePoolName, len(state.InstancesByMig)),
		}}),
	}, nil
}

func (a *nodePoolTerminateInstancesAttack) Start(ctx context.Context, state *NodePoolTerminateInstancesState) (*action_kit_api.StartResult, error) {
	if len(state.InstancesByMig) == 0 {
		return nil, extension_kit.ToError("No instances selected for termination.", nil)
	}
	client, closer, err := a.migClientProvider(ctx, state.ProjectID)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to create MIG client for project %s", state.ProjectID), err)
	}
	defer closer()
	total := 0
	for key, urls := range state.InstancesByMig {
		zone, name, _ := strings.Cut(key, "/")
		_, err := client.DeleteInstances(ctx, &computepb.DeleteInstancesInstanceGroupManagerRequest{
			Project:              state.ProjectID,
			Zone:                 zone,
			InstanceGroupManager: name,
			InstanceGroupManagersDeleteInstancesRequestResource: &computepb.InstanceGroupManagersDeleteInstancesRequest{
				Instances: urls,
			},
		})
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to delete instances from MIG %s/%s", zone, name), err)
		}
		total += len(urls)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Deletion requested for %d instance(s) in GKE node pool %s/%s. GKE will replace them via the underlying MIG.", total, state.ClusterName, state.NodePoolName),
		}}),
	}, nil
}

// parseZonalMIGUrl extracts (zone, name) from a Compute Engine InstanceGroupManager URL like
// https://www.googleapis.com/compute/v1/projects/<project>/zones/<zone>/instanceGroupManagers/<name>.
func parseZonalMIGUrl(url string) (zone string, name string, ok bool) {
	const zonesMarker = "/zones/"
	const igmMarker = "/instanceGroupManagers/"
	zIdx := strings.Index(url, zonesMarker)
	iIdx := strings.Index(url, igmMarker)
	if zIdx < 0 || iIdx <= zIdx {
		return "", "", false
	}
	zone = url[zIdx+len(zonesMarker) : iIdx]
	name = url[iIdx+len(igmMarker):]
	if zone == "" || name == "" {
		return "", "", false
	}
	return zone, name, true
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
