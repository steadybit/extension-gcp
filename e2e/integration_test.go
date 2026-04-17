// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-gcp/extvm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mockProjectID = "mock-project"
	mockVMName    = "test"
	mockZone      = "us-central1-a"
)

func TestWithMinikube(t *testing.T) {
	server := startMockComputeServer()
	defer server.Close()

	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-gcp",
		Port: 8093,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=debug",
				"--set", "gcp.projectID=" + mockProjectID,
				"--set", "testing.computeEndpoint=http://host.minikube.internal:" + server.hostPort(),
			}
		},
	}

	e2e.WithMinikube(t, e2e.DefaultMinikubeOpts(), &extFactory, []e2e.WithMinikubeTestCase{
		{Name: "discovery", Test: testDiscovery},
		{Name: "stop action", Test: testStopAction(server)},
	})
}

func testDiscovery(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
	log.Info().Msg("Starting testDiscovery")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	target, err := e2e.PollForTarget(ctx, e, extvm.TargetIDVM, func(target discovery_kit_api.Target) bool {
		return e2e.HasAttribute(target, "gcp-vm.name", mockVMName)
	})
	require.NoError(t, err)
	assert.Equal(t, extvm.TargetIDVM, target.TargetType)
	assert.True(t, e2e.HasAttribute(target, "gcp.project.id", mockProjectID))
	assert.True(t, e2e.HasAttribute(target, "gcp.zone", mockZone))
	assert.True(t, e2e.HasAttribute(target, "gcp-vm.id", "42"))
}

func testStopAction(server *mockComputeServer) func(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	return func(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
		log.Info().Msg("Starting testStopAction")
		before := len(server.StopRequests())

		exec, err := e.RunAction(
			extvm.VirtualMachineStateActionId,
			&action_kit_api.Target{
				Name: mockVMName,
				Attributes: map[string][]string{
					"gcp-vm.name":    {mockVMName},
					"gcp.zone":       {mockZone},
					"gcp.project.id": {mockProjectID},
				},
			},
			map[string]any{"action": "stop"},
			nil,
		)
		require.NoError(t, err)
		require.NoError(t, exec.Wait())

		require.Eventually(t, func() bool {
			return len(server.StopRequests()) > before
		}, 10*time.Second, 200*time.Millisecond, "mock server did not record a stop request")

		got := server.StopRequests()[len(server.StopRequests())-1]
		assert.Equal(t, instanceRef{Project: mockProjectID, Zone: mockZone, Instance: mockVMName}, got)
	}
}
