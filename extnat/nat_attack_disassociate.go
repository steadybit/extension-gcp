/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extnat

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-gcp/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/protobuf/proto"
)

// CloudNatDisassociateState captures the entire original NAT proto so the
// attack can restore it byte-identically on stop. We serialise via proto.Marshal
// (base64 in JSON) rather than snapshotting individual fields because a
// LIST_OF_SUBNETWORKS NAT can't survive an empty subnetworks list — the only
// way to disassociate a NAT from all its subnetworks in GCP is to remove the
// NAT entry from the router entirely, then re-add it.
type CloudNatDisassociateState struct {
	ProjectID   string
	Region      string
	RouterName  string
	NatName     string
	NatSnapshot []byte // proto.Marshal of the original computepb.RouterNat
	// SourceMode is the NAT's sourceSubnetworkIpRangesToNat mode, captured for
	// human-readable Prepare/Start/Stop messages. Set to the literal string
	// GCP returned (e.g. LIST_OF_SUBNETWORKS, ALL_SUBNETWORKS_ALL_IP_RANGES).
	SourceMode string
	// SubnetCount is only meaningful when SourceMode is LIST_OF_SUBNETWORKS —
	// the other modes NAT every subnet in the region without populating an
	// explicit list, so this field stays 0 there.
	SubnetCount int
}

type cloudNatDisassociateAttack struct {
	clientProvider func(ctx context.Context, projectID string) (*compute.RoutersClient, func(), error)
}

var _ action_kit_sdk.Action[CloudNatDisassociateState] = (*cloudNatDisassociateAttack)(nil)
var _ action_kit_sdk.ActionWithStop[CloudNatDisassociateState] = (*cloudNatDisassociateAttack)(nil)

func NewCloudNatDisassociateAction() action_kit_sdk.ActionWithStop[CloudNatDisassociateState] {
	return &cloudNatDisassociateAttack{
		clientProvider: func(ctx context.Context, projectID string) (*compute.RoutersClient, func(), error) {
			access, err := utils.GetGcpAccess(projectID)
			if err != nil {
				return nil, nil, err
			}
			c, err := compute.NewRoutersRESTClient(ctx, access.ClientOptions...)
			if err != nil {
				return nil, nil, err
			}
			return c, func() { _ = c.Close() }, nil
		},
	}
}

func (a *cloudNatDisassociateAttack) NewEmptyState() CloudNatDisassociateState {
	return CloudNatDisassociateState{}
}

func (a *cloudNatDisassociateAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          CloudNatDisassociateActionId,
		Label:       "Suspend Cloud NAT",
		Description: "Removes the Cloud NAT so VMs in its subnetworks lose internet egress. Restored on stop.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDCloudNat,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by router + NAT name",
					Description: extutil.Ptr("Find Cloud NAT by router name and NAT name"),
					Query:       "gcp.cloud-nat.router=\"\" and gcp.cloud-nat.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("GCP"),
		Category:    extutil.Ptr("Cloud NAT"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long the Cloud NAT stays removed. Restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *cloudNatDisassociateAttack) Prepare(ctx context.Context, state *CloudNatDisassociateState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	if err := populatePrepareTarget(state, request); err != nil {
		return nil, err
	}
	router, err := fetchRouter(ctx, a.clientProvider, state)
	if err != nil {
		return nil, err
	}
	nat := findNat(router, state.NatName)
	if nat == nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Cloud NAT %s/%s not found on router — cannot disassociate", state.RouterName, state.NatName), nil)
	}
	blob, err := proto.Marshal(nat)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to snapshot Cloud NAT %s/%s config", state.RouterName, state.NatName), err)
	}
	state.NatSnapshot = blob
	state.SourceMode = nat.GetSourceSubnetworkIpRangesToNat()
	state.SubnetCount = len(nat.GetSubnetworks())
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Will remove Cloud NAT %s/%s (%s)", state.RouterName, state.NatName, natCoverageDescription(state.SourceMode, state.SubnetCount)),
		}}),
	}, nil
}

// natCoverageDescription renders a human-readable summary of what the NAT
// covers. For LIST_OF_SUBNETWORKS mode we can say "N subnetwork(s)"; the
// ALL_SUBNETWORKS_* modes don't populate an explicit list, so the subnet
// count alone would misleadingly read as zero.
func natCoverageDescription(sourceMode string, subnetCount int) string {
	switch sourceMode {
	case "LIST_OF_SUBNETWORKS":
		return fmt.Sprintf("attached to %d subnetwork(s)", subnetCount)
	case "ALL_SUBNETWORKS_ALL_IP_RANGES":
		return "NATs every subnetwork in the region"
	case "ALL_SUBNETWORKS_ALL_PRIMARY_IP_RANGES":
		return "NATs the primary IP range of every subnetwork in the region"
	default:
		return fmt.Sprintf("mode=%s", sourceMode)
	}
}

func populatePrepareTarget(state *CloudNatDisassociateState, request action_kit_api.PrepareActionRequestBody) error {
	state.ProjectID = mustHave(request.Target.Attributes, "gcp.project.id")
	state.Region = mustHave(request.Target.Attributes, "gcp.cloud-nat.region")
	state.RouterName = mustHave(request.Target.Attributes, "gcp.cloud-nat.router")
	state.NatName = mustHave(request.Target.Attributes, "gcp.cloud-nat.name")
	if state.ProjectID == "" || state.Region == "" || state.RouterName == "" || state.NatName == "" {
		return extension_kit.ToError("Target is missing one of: gcp.project.id, gcp.cloud-nat.region, gcp.cloud-nat.router, gcp.cloud-nat.name", nil)
	}
	return nil
}

func fetchRouter(ctx context.Context, provider func(ctx context.Context, projectID string) (*compute.RoutersClient, func(), error), state *CloudNatDisassociateState) (*computepb.Router, error) {
	client, closer, err := provider(ctx, state.ProjectID)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to create Routers client for project %s", state.ProjectID), err)
	}
	defer closer()
	router, err := client.Get(ctx, &computepb.GetRouterRequest{Project: state.ProjectID, Region: state.Region, Router: state.RouterName})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to get router %s/%s", state.Region, state.RouterName), err)
	}
	return router, nil
}

// findNat returns the NAT with the given name from the router, or nil if
// absent. Cloud NAT config lives as a repeated field on the router, so we
// scan by name.
func findNat(router *computepb.Router, natName string) *computepb.RouterNat {
	for _, nat := range router.GetNats() {
		if nat.GetName() == natName {
			return nat
		}
	}
	return nil
}

func (a *cloudNatDisassociateAttack) Start(ctx context.Context, state *CloudNatDisassociateState) (*action_kit_api.StartResult, error) {
	if err := removeNat(ctx, a.clientProvider, state); err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to remove Cloud NAT %s/%s", state.RouterName, state.NatName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Removed Cloud NAT %s/%s (%s); traffic in those subnets loses NAT egress until Stop restores it", state.RouterName, state.NatName, natCoverageDescription(state.SourceMode, state.SubnetCount)),
		}}),
	}, nil
}

func (a *cloudNatDisassociateAttack) Stop(ctx context.Context, state *CloudNatDisassociateState) (*action_kit_api.StopResult, error) {
	if err := restoreNat(ctx, a.clientProvider, state); err != nil {
		log.Error().Err(err).Msgf("Failed to restore Cloud NAT %s/%s", state.RouterName, state.NatName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore Cloud NAT %s/%s", state.RouterName, state.NatName), err)
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Restored Cloud NAT %s/%s (%s)", state.RouterName, state.NatName, natCoverageDescription(state.SourceMode, state.SubnetCount)),
		}}),
	}, nil
}

// removeNat fetches the router, drops the target NAT from its Nats[] list,
// and PATCHes. Other NATs sharing the router stay untouched. Re-fetch on
// every call to survive concurrent edits to sibling NATs.
//
// Idempotent: if the NAT is already gone (Start retried after a successful
// removal), we return nil rather than erroring — the desired end state
// (NAT absent) is already achieved. Mirrors restoreNat's replace-or-append
// symmetry.
func removeNat(ctx context.Context, provider func(ctx context.Context, projectID string) (*compute.RoutersClient, func(), error), state *CloudNatDisassociateState) error {
	client, closer, err := provider(ctx, state.ProjectID)
	if err != nil {
		return err
	}
	defer closer()
	router, err := client.Get(ctx, &computepb.GetRouterRequest{Project: state.ProjectID, Region: state.Region, Router: state.RouterName})
	if err != nil {
		return fmt.Errorf("get router: %w", err)
	}
	kept := make([]*computepb.RouterNat, 0, len(router.GetNats()))
	found := false
	for _, nat := range router.GetNats() {
		if nat.GetName() == state.NatName {
			found = true
			continue
		}
		kept = append(kept, nat)
	}
	if !found {
		// Already removed by a prior Start invocation — nothing to do.
		log.Info().Msgf("Cloud NAT %s/%s already absent — treating remove as no-op success", state.RouterName, state.NatName)
		return nil
	}
	router.Nats = kept
	_, err = client.Patch(ctx, &computepb.PatchRouterRequest{
		Project:        state.ProjectID,
		Region:         state.Region,
		Router:         state.RouterName,
		RouterResource: router,
	})
	return err
}

// restoreNat fetches the router and re-appends the snapshotted NAT. If a NAT
// with the same name already exists (e.g. Stop retried after a partial
// success), replace it with the snapshot rather than duplicating.
func restoreNat(ctx context.Context, provider func(ctx context.Context, projectID string) (*compute.RoutersClient, func(), error), state *CloudNatDisassociateState) error {
	if len(state.NatSnapshot) == 0 {
		return fmt.Errorf("no NAT snapshot to restore")
	}
	original := &computepb.RouterNat{}
	if err := proto.Unmarshal(state.NatSnapshot, original); err != nil {
		return fmt.Errorf("unmarshal NAT snapshot: %w", err)
	}
	client, closer, err := provider(ctx, state.ProjectID)
	if err != nil {
		return err
	}
	defer closer()
	router, err := client.Get(ctx, &computepb.GetRouterRequest{Project: state.ProjectID, Region: state.Region, Router: state.RouterName})
	if err != nil {
		return fmt.Errorf("get router: %w", err)
	}
	replaced := false
	for i, nat := range router.GetNats() {
		if nat.GetName() == state.NatName {
			router.Nats[i] = original
			replaced = true
			break
		}
	}
	if !replaced {
		router.Nats = append(router.Nats, original)
	}
	_, err = client.Patch(ctx, &computepb.PatchRouterRequest{
		Project:        state.ProjectID,
		Region:         state.Region,
		Router:         state.RouterName,
		RouterResource: router,
	})
	return err
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
