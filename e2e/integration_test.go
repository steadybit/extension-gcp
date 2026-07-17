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
	"github.com/steadybit/extension-gcp/extcloudsql"
	"github.com/steadybit/extension-gcp/extdisk"
	"github.com/steadybit/extension-gcp/extmig"
	"github.com/steadybit/extension-gcp/extnat"
	"github.com/steadybit/extension-gcp/extvm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mockProjectID = "mock-project"
	mockVMName    = "test"
	mockZone      = "us-central1-a"
)

// discoveryCase describes a discovery e2e test: poll for a target of targetID matching matchAttr==matchVal,
// then assert the target type plus the want attributes. The mock backend for each resource lives in
// mock_gcp_server_test.go.
type discoveryCase struct {
	name      string
	targetID  string
	matchAttr string
	matchVal  string
	want      map[string]string
}

var discoveryCases = []discoveryCase{
	{"vm discovery", extvm.TargetIDVM, "gcp-vm.name", mockVMName, map[string]string{
		"gcp.project.id": mockProjectID, "gcp.zone": mockZone, "gcp-vm.id": "42"}},
	{"cloud-nat discovery", extnat.TargetIDCloudNat, "gcp.cloud-nat.name", "mock-nat", map[string]string{
		"gcp.project.id": mockProjectID, "gcp.cloud-nat.router": "mock-router", "gcp.cloud-nat.region": "us-central1"}},
	{"mig discovery", extmig.TargetIDMig, "gcp.mig.name", "mock-mig", map[string]string{
		"gcp.project.id": mockProjectID, "gcp.mig.scope": "zonal", "gcp.mig.location": mockZone, "gcp.mig.target-size": "3"}},
	{"persistent-disk discovery", extdisk.TargetIDDisk, "gcp.persistent-disk.name", "mock-disk", map[string]string{
		"gcp.project.id": mockProjectID, "gcp.persistent-disk.zone": mockZone, "gcp.persistent-disk.type": "pd-ssd", "gcp.persistent-disk.size-gb": "100"}},
	{"cloud-sql discovery", extcloudsql.TargetIDInstance, "gcp.cloudsql.instance.name", "mock-sql", map[string]string{
		"gcp.project.id": mockProjectID, "gcp.cloudsql.region": "us-central1", "gcp.cloudsql.database-version": "POSTGRES_15", "gcp.cloudsql.tier": "db-custom-1-3840"}},
}

func TestWithMinikube(t *testing.T) {
	server := startMockGcpServer()
	defer server.Close()

	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-gcp",
		Port: 8093,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=debug",
				"--set", "gcp.projectID=" + mockProjectID,
				"--set", "testing.computeEndpoint=http://host.minikube.internal:" + server.port(),
				"--set", "discovery.enable.cloudNat=true",
				"--set", "discovery.enable.mig=true",
				"--set", "discovery.enable.persistentDisk=true",
				"--set", "discovery.enable.cloudSql=true",
			}
		},
	}

	testCases := make([]e2e.WithMinikubeTestCase, 0, len(discoveryCases)+1)
	for _, tc := range discoveryCases {
		testCases = append(testCases, e2e.WithMinikubeTestCase{Name: tc.name, Test: testDiscovery(tc)})
	}
	testCases = append(testCases, e2e.WithMinikubeTestCase{Name: "stop action", Test: testStopAction(server)})

	e2e.WithMinikube(t, e2e.DefaultMinikubeOpts(), &extFactory, testCases)
}

func testDiscovery(tc discoveryCase) func(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	return func(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
		log.Info().Msgf("Starting discovery test %q", tc.name)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		target, err := e2e.PollForTarget(ctx, e, tc.targetID, func(target discovery_kit_api.Target) bool {
			return e2e.HasAttribute(target, tc.matchAttr, tc.matchVal)
		})
		require.NoError(t, err)
		assert.Equal(t, tc.targetID, target.TargetType)
		for k, v := range tc.want {
			assert.True(t, e2e.HasAttribute(target, k, v), "expected attribute %s=%s", k, v)
		}
	}
}

func testStopAction(server *mockGcpServer) func(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
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
		// The action is instantaneous (no status poll, no stop hook) so exec.Wait() would
		// block forever. Cancelling releases the harness goroutine immediately.
		defer func() { _ = exec.Cancel() }()

		require.Eventually(t, func() bool {
			return len(server.StopRequests()) > before
		}, 10*time.Second, 200*time.Millisecond, "mock server did not record a stop request")

		got := server.StopRequests()[len(server.StopRequests())-1]
		assert.Equal(t, instanceRef{Project: mockProjectID, Zone: mockZone, Instance: mockVMName}, got)
	}
}
