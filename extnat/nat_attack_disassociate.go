/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
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
)

// CloudNatDisassociateState captures the original subnetworks of the target Cloud NAT so we can restore
// them on stop. We only mutate the NAT we own; other NATs sharing the same router are preserved verbatim.
type CloudNatDisassociateState struct {
	ProjectID  string
	Region     string
	RouterName string
	NatName    string
	// OriginalSubnetworks are protobuf-serializable copies of the NAT's original subnetwork entries.
	OriginalSubnetworks []natSubnetSnapshot
}

// natSubnetSnapshot mirrors the parts of computepb.RouterNatSubnetworkToNat that the attack restores.
type natSubnetSnapshot struct {
	Name                  string
	SecondaryIPRangeNames []string
	SourceIPRangesToNat   []string
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
		Id:    CloudNatDisassociateActionId,
		Label: "Disassociate Cloud NAT from its subnetworks",
		Description: "Removes all subnetworks from the Cloud NAT's configuration so VMs in those subnets lose their NAT egress to the internet. " +
			"Subnetworks are restored on stop. Other NATs sharing the same router are preserved untouched.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
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
		Technology:  extutil.Ptr("Google Cloud"),
		Category:    extutil.Ptr("Cloud NAT"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long subnetworks remain disassociated from the Cloud NAT. Restored on stop."),
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
	state.ProjectID = mustHave(request.Target.Attributes, "gcp.project.id")
	state.Region = mustHave(request.Target.Attributes, "gcp.cloud-nat.region")
	state.RouterName = mustHave(request.Target.Attributes, "gcp.cloud-nat.router")
	state.NatName = mustHave(request.Target.Attributes, "gcp.cloud-nat.name")
	if state.ProjectID == "" || state.Region == "" || state.RouterName == "" || state.NatName == "" {
		return nil, extension_kit.ToError("Target is missing one of: gcp.project.id, gcp.cloud-nat.region, gcp.cloud-nat.router, gcp.cloud-nat.name", nil)
	}
	client, closer, err := a.clientProvider(ctx, state.ProjectID)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to create Routers client for project %s", state.ProjectID), err)
	}
	defer closer()
	router, err := client.Get(ctx, &computepb.GetRouterRequest{Project: state.ProjectID, Region: state.Region, Router: state.RouterName})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to get router %s/%s", state.Region, state.RouterName), err)
	}
	for _, nat := range router.GetNats() {
		if nat.GetName() != state.NatName {
			continue
		}
		for _, s := range nat.GetSubnetworks() {
			snap := natSubnetSnapshot{}
			if s.Name != nil {
				snap.Name = *s.Name
			}
			snap.SecondaryIPRangeNames = append(snap.SecondaryIPRangeNames, s.GetSecondaryIpRangeNames()...)
			snap.SourceIPRangesToNat = append(snap.SourceIPRangesToNat, s.GetSourceIpRangesToNat()...)
			state.OriginalSubnetworks = append(state.OriginalSubnetworks, snap)
		}
		break
	}
	if len(state.OriginalSubnetworks) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("Cloud NAT %s/%s has no subnetworks to disassociate", state.RouterName, state.NatName), nil)
	}
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Will disassociate %d subnetwork(s) from Cloud NAT %s/%s", len(state.OriginalSubnetworks), state.RouterName, state.NatName),
		}}),
	}, nil
}

func (a *cloudNatDisassociateAttack) Start(ctx context.Context, state *CloudNatDisassociateState) (*action_kit_api.StartResult, error) {
	if err := setNatSubnetworks(ctx, a.clientProvider, state, nil); err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to disassociate Cloud NAT %s/%s subnetworks", state.RouterName, state.NatName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Disassociated %d subnetwork(s) from Cloud NAT %s/%s", len(state.OriginalSubnetworks), state.RouterName, state.NatName),
		}}),
	}, nil
}

func (a *cloudNatDisassociateAttack) Stop(ctx context.Context, state *CloudNatDisassociateState) (*action_kit_api.StopResult, error) {
	if err := setNatSubnetworks(ctx, a.clientProvider, state, state.OriginalSubnetworks); err != nil {
		log.Error().Err(err).Msgf("Failed to restore Cloud NAT %s/%s subnetworks", state.RouterName, state.NatName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore Cloud NAT %s/%s subnetworks", state.RouterName, state.NatName), err)
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Restored %d subnetwork(s) on Cloud NAT %s/%s", len(state.OriginalSubnetworks), state.RouterName, state.NatName),
		}}),
	}, nil
}

// setNatSubnetworks re-fetches the router and PATCHes it with the target NAT's subnetworks set to the
// given snapshot list. Other NATs on the router are preserved verbatim. Re-fetch protects concurrent edits
// to other NATs (or the same NAT's other fields like nat-ips) — we only own the subnetworks list of one NAT.
func setNatSubnetworks(ctx context.Context, provider func(ctx context.Context, projectID string) (*compute.RoutersClient, func(), error), state *CloudNatDisassociateState, target []natSubnetSnapshot) error {
	client, closer, err := provider(ctx, state.ProjectID)
	if err != nil {
		return err
	}
	defer closer()
	router, err := client.Get(ctx, &computepb.GetRouterRequest{Project: state.ProjectID, Region: state.Region, Router: state.RouterName})
	if err != nil {
		return fmt.Errorf("get router: %w", err)
	}
	for _, nat := range router.GetNats() {
		if nat.GetName() != state.NatName {
			continue
		}
		nat.Subnetworks = toSubnetworkProtos(target)
		break
	}
	_, err = client.Patch(ctx, &computepb.PatchRouterRequest{
		Project:        state.ProjectID,
		Region:         state.Region,
		Router:         state.RouterName,
		RouterResource: router,
	})
	return err
}

func toSubnetworkProtos(snapshots []natSubnetSnapshot) []*computepb.RouterNatSubnetworkToNat {
	if len(snapshots) == 0 {
		return []*computepb.RouterNatSubnetworkToNat{}
	}
	out := make([]*computepb.RouterNatSubnetworkToNat, 0, len(snapshots))
	for i := range snapshots {
		s := snapshots[i]
		entry := &computepb.RouterNatSubnetworkToNat{}
		if s.Name != "" {
			n := s.Name
			entry.Name = &n
		}
		if len(s.SecondaryIPRangeNames) > 0 {
			entry.SecondaryIpRangeNames = append([]string(nil), s.SecondaryIPRangeNames...)
		}
		if len(s.SourceIPRangesToNat) > 0 {
			entry.SourceIpRangesToNat = append([]string(nil), s.SourceIPRangesToNat...)
		}
		out = append(out, entry)
	}
	return out
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
