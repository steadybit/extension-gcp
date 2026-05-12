/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extcloudsql

import (
	"context"
	"fmt"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-gcp/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/sqladmin/v1"
)

// CloudSqlFailoverState holds enough to trigger a failover. The attack is instantaneous — there is no
// automatic rollback; running a second failover restores the original primary. Cloud SQL only supports
// failover on HA (REGIONAL availability_type) instances.
type CloudSqlFailoverState struct {
	ProjectID    string
	InstanceName string
}

type cloudSqlFailoverAttack struct {
	clientProvider func(ctx context.Context, projectID string) (*sqladmin.Service, error)
}

var _ action_kit_sdk.Action[CloudSqlFailoverState] = (*cloudSqlFailoverAttack)(nil)

func NewInstanceFailoverAction() action_kit_sdk.Action[CloudSqlFailoverState] {
	return &cloudSqlFailoverAttack{
		clientProvider: func(ctx context.Context, projectID string) (*sqladmin.Service, error) {
			access, err := utils.GetGcpAccess(projectID)
			if err != nil {
				return nil, err
			}
			return sqladmin.NewService(ctx, access.ClientOptions...)
		},
	}
}

func (a *cloudSqlFailoverAttack) NewEmptyState() CloudSqlFailoverState {
	return CloudSqlFailoverState{}
}

func (a *cloudSqlFailoverAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    InstanceFailoverActionId,
		Label: "Trigger Cloud SQL failover",
		Description: "Triggers a failover from the primary instance to its REGIONAL standby. Only works on Cloud SQL instances with availability-type=REGIONAL (HA). " +
			"Validates that your application correctly handles the brief connection interruption and follows the secondary's new primary role. " +
			"There is no automatic rollback; running a second failover swaps the roles back if desired.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDInstance,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by Cloud SQL instance name",
					Description: extutil.Ptr("Find Cloud SQL instance by name"),
					Query:       "gcp.cloudsql.instance.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Google Cloud"),
		Category:    extutil.Ptr("Cloud SQL"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (a *cloudSqlFailoverAttack) Prepare(_ context.Context, state *CloudSqlFailoverState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.ProjectID = mustHave(request.Target.Attributes, "gcp.project.id")
	state.InstanceName = mustHave(request.Target.Attributes, "gcp.cloudsql.instance.name")
	if state.ProjectID == "" || state.InstanceName == "" {
		return nil, extension_kit.ToError("Target is missing one of: gcp.project.id, gcp.cloudsql.instance.name", nil)
	}
	if availability := request.Target.Attributes["gcp.cloudsql.availability-type"]; len(availability) == 0 || availability[0] != "REGIONAL" {
		return nil, extension_kit.ToError(fmt.Sprintf("Cloud SQL instance %s is not REGIONAL (HA); failover requires a high-availability instance", state.InstanceName), nil)
	}
	return nil, nil
}

func (a *cloudSqlFailoverAttack) Start(ctx context.Context, state *CloudSqlFailoverState) (*action_kit_api.StartResult, error) {
	svc, err := a.clientProvider(ctx, state.ProjectID)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Cloud SQL client for project %s", state.ProjectID), err)
	}
	_, err = svc.Instances.Failover(state.ProjectID, state.InstanceName, &sqladmin.InstancesFailoverRequest{}).Context(ctx).Do()
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to trigger Cloud SQL failover for %s", state.InstanceName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Failover triggered for Cloud SQL instance %s", state.InstanceName),
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
