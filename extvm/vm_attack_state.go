/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extvm

import (
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"fmt"
	"github.com/googleapis/gax-go/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type virtualMachineStateAction struct {
	clientProvider func(ctx context.Context) (virtualMachineStateChangeApi, error)
}

var _ action_kit_sdk.Action[VirtualMachineStateChangeState] = (*virtualMachineStateAction)(nil)

type VirtualMachineStateChangeState struct {
	ProjectId string
	VmName    string
	Zone      string
	Action    string
}

type virtualMachineStateChangeApi interface {
	Delete(ctx context.Context, req *computepb.DeleteInstanceRequest, opts ...gax.CallOption) (*compute.Operation, error)
	Stop(ctx context.Context, req *computepb.StopInstanceRequest, opts ...gax.CallOption) (*compute.Operation, error)
	Reset(ctx context.Context, req *computepb.ResetInstanceRequest, opts ...gax.CallOption) (*compute.Operation, error)
	Suspend(ctx context.Context, req *computepb.SuspendInstanceRequest, opts ...gax.CallOption) (*compute.Operation, error)
}

func NewVirtualMachineStateAction() action_kit_sdk.Action[VirtualMachineStateChangeState] {
	return &virtualMachineStateAction{defaultClientProvider}
}

func (e *virtualMachineStateAction) NewEmptyState() VirtualMachineStateChangeState {
	return VirtualMachineStateChangeState{}
}

func (e *virtualMachineStateAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          VirtualMachineStateActionId,
		Label:       "Change Virtual Machine State",
		Description: "Reset, stop, suspend or delete Google Cloud virtual machines",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDVM,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by vm-name",
					Description: extutil.Ptr("Find gcp virtual machine by name"),
					Query:       "gcp-vm.name=\"\"",
				},
				{
					Label:       "by cluster name",
					Description: extutil.Ptr("Find gcp virtual machine by cluster name"),
					Query:       "gcp-kubernetes-engine.cluster.name=\"\"",
				},
			}),
		}),
		Category:    extutil.Ptr("state"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:        "action",
				Label:       "Action",
				Description: extutil.Ptr("The kind of state change operation to execute for the gcp virtual machines"),
				Required:    extutil.Ptr(true),
				Type:        action_kit_api.String,
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Reset",
						Value: "reset",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Stop",
						Value: "stop",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Delete",
						Value: "delete",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Suspend",
						Value: "suspend",
					},
				}),
			},
		},
	}
}

func (e *virtualMachineStateAction) Prepare(_ context.Context, state *VirtualMachineStateChangeState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	vmName := request.Target.Attributes["gcp-vm.name"]
	if len(vmName) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'gcp-vm.name' attribute.", nil)
	}

	zone := request.Target.Attributes["gcp.zone"]
	if len(zone) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'gcp.zone' attribute.", nil)
	}

	projectId := request.Target.Attributes["gcp.project.id"]
	if len(projectId) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'gcp.project.id' attribute.", nil)
	}

	action := request.Config["action"]
	if action == nil {
		return nil, extension_kit.ToError("Missing attack action parameter.", nil)
	}

	state.Zone = zone[0]
	state.VmName = vmName[0]
	state.ProjectId = projectId[0]
	state.Action = action.(string)
	return nil, nil
}

func (e *virtualMachineStateAction) Start(ctx context.Context, state *VirtualMachineStateChangeState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(ctx)
	if err != nil {
		return nil, extension_kit.ToError("Failed to initialize gcp client", err)
	}

	if state.Action == "reset" {
		_, err = client.Reset(ctx, &computepb.ResetInstanceRequest{
			Zone:     state.Zone,
			Project:  state.ProjectId,
			Instance: state.VmName,
		})
	} else if state.Action == "stop" {
		_, err = client.Stop(ctx, &computepb.StopInstanceRequest{
			Zone:     state.Zone,
			Project:  state.ProjectId,
			Instance: state.VmName,
		})
	} else if state.Action == "delete" {
		_, err = client.Delete(ctx, &computepb.DeleteInstanceRequest{
			Zone:     state.Zone,
			Project:  state.ProjectId,
			Instance: state.VmName,
		})
	} else if state.Action == "suspend" {
		_, err = client.Suspend(ctx, &computepb.SuspendInstanceRequest{
			Zone:     state.Zone,
			Project:  state.ProjectId,
			Instance: state.VmName,
		})
	} else {
		return nil, extension_kit.ToError(fmt.Sprintf("Unknown state change attack '%s'", state.Action), nil)
	}

	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to execute state change attack '%s' on vm '%s'", state.Action, state.VmName), err)
	}

	return nil, nil
}

func defaultClientProvider(ctx context.Context) (virtualMachineStateChangeApi, error) {
	return GetGcpInstancesClient(ctx)
}
