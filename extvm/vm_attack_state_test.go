package extvm

import (
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"errors"
	"github.com/googleapis/gax-go/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGcpVirtualMachineStateAction_Prepare(t *testing.T) {
	action := virtualMachineStateAction{}

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError error
		wantedState *VirtualMachineStateChangeState
	}{
		{
			name: "Should return config",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "stop",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"gcp-vm.name":    {"my-vm"},
						"gcp.project.id": {"42"},
						"gcp.zone":       {"us-central1-a	"},
					},
				}),
			}),

			wantedState: &VirtualMachineStateChangeState{
				VmName:    "my-vm",
				Action:    "stop",
				ProjectId: "42",
				Zone:      "us-central1-a	",
			},
		},
		{
			name: "Should return error if project id is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "stop",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"gcp-vm.name": {"my-vm"},
						"gcp.zone":    {"us-central1-a	"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'gcp.project.id' attribute.", nil),
		},
		{
			name: "Should return error if vm name is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "stop",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"gcp.project.id": {"42"},
						"gcp.zone":       {"us-central1-a	"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'gcp-vm.name' attribute.", nil),
		},
		{
			name: "Should return error if zone is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "stop",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"gcp-vm.name":    {"my-vm"},
						"gcp.project.id": {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'gcp.zone' attribute.", nil),
		},
		{
			name: "Should return error if action is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"gcp-vm.name":    {"my-vm"},
						"gcp.project.id": {"42"},
						"gcp.zone":       {"us-central1-a	"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Missing attack action parameter.", nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := action.NewEmptyState()
			request := tt.requestBody
			//When
			_, err := action.Prepare(context.Background(), &state, request)

			//Then
			if tt.wantedError != nil {
				assert.EqualError(t, err, tt.wantedError.Error())
			}
			if tt.wantedState != nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantedState.Zone, state.Zone)
				assert.Equal(t, tt.wantedState.VmName, state.VmName)
				assert.Equal(t, tt.wantedState.ProjectId, state.ProjectId)
				assert.EqualValues(t, tt.wantedState.Action, state.Action)
			}
		})
	}
}

type gcpClientApiMock struct {
	mock.Mock
}

func (m *gcpClientApiMock) Delete(ctx context.Context, req *computepb.DeleteInstanceRequest, _ ...gax.CallOption) (*compute.Operation, error) {
	args := m.Called(ctx, req)
	return nil, args.Error(1)
}

func (m *gcpClientApiMock) Stop(ctx context.Context, req *computepb.StopInstanceRequest, _ ...gax.CallOption) (*compute.Operation, error) {
	args := m.Called(ctx, req)
	return nil, args.Error(1)
}

func (m *gcpClientApiMock) Reset(ctx context.Context, req *computepb.ResetInstanceRequest, _ ...gax.CallOption) (*compute.Operation, error) {
	args := m.Called(ctx, req)
	return nil, args.Error(1)
}
func (m *gcpClientApiMock) Suspend(ctx context.Context, req *computepb.SuspendInstanceRequest, _ ...gax.CallOption) (*compute.Operation, error) {
	args := m.Called(ctx, req)
	return nil, args.Error(1)
}

func TestGcpVirtualMachineStateAction_Suspend(t *testing.T) {
	// Given
	api := new(gcpClientApiMock)
	api.On("Suspend", mock.Anything, mock.MatchedBy(func(req *computepb.SuspendInstanceRequest) bool {
		require.Equal(t, "42", req.Project)
		require.Equal(t, "us-central1-a", req.Zone)
		require.Equal(t, "my-vm", req.Instance)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(ctx context.Context) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		ProjectId: "42",
		VmName:    "my-vm",
		Zone:      "us-central1-a",
		Action:    "suspend",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestGcpVirtualMachineStateAction_Delete(t *testing.T) {
	// Given
	api := new(gcpClientApiMock)
	api.On("Delete", mock.Anything, mock.MatchedBy(func(req *computepb.DeleteInstanceRequest) bool {
		require.Equal(t, "42", req.Project)
		require.Equal(t, "us-central1-a", req.Zone)
		require.Equal(t, "my-vm", req.Instance)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(ctx context.Context) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		ProjectId: "42",
		VmName:    "my-vm",
		Zone:      "us-central1-a",
		Action:    "delete",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestGcpVirtualMachineStateAction_Stop(t *testing.T) {
	// Given
	api := new(gcpClientApiMock)
	api.On("Stop", mock.Anything, mock.MatchedBy(func(req *computepb.StopInstanceRequest) bool {
		require.Equal(t, "42", req.Project)
		require.Equal(t, "us-central1-a", req.Zone)
		require.Equal(t, "my-vm", req.Instance)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(ctx context.Context) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		ProjectId: "42",
		VmName:    "my-vm",
		Zone:      "us-central1-a",
		Action:    "stop",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestGcpVirtualMachineStateAction_Reset(t *testing.T) {
	// Given
	api := new(gcpClientApiMock)
	api.On("Reset", mock.Anything, mock.MatchedBy(func(req *computepb.ResetInstanceRequest) bool {
		require.Equal(t, "42", req.Project)
		require.Equal(t, "us-central1-a", req.Zone)
		require.Equal(t, "my-vm", req.Instance)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(ctx context.Context) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		ProjectId: "42",
		VmName:    "my-vm",
		Zone:      "us-central1-a",
		Action:    "reset",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestStartVirtualMachineStateChangeForwardsError(t *testing.T) {
	// Given
	api := new(gcpClientApiMock)
	api.On("Stop", mock.Anything, mock.MatchedBy(func(req *computepb.StopInstanceRequest) bool {
		require.Equal(t, "42", req.Project)
		require.Equal(t, "us-central1-a", req.Zone)
		require.Equal(t, "my-vm", req.Instance)
		return true
	})).Return(nil, errors.New("expected"))
	action := virtualMachineStateAction{clientProvider: func(ctx context.Context) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		ProjectId: "42",
		VmName:    "my-vm",
		Zone:      "us-central1-a",
		Action:    "stop",
	})

	// Then
	assert.Error(t, err, "Failed to execute state change attack")
	assert.Nil(t, result)

	api.AssertExpectations(t)
}
