// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-gcp/extvm"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-gcp",
		Port: 8093,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=debug",
			}
		},
	}

	e2e.WithMinikube(t, e2e.DefaultMiniKubeOpts(), &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "discovery",
			Test: testDiscovery,
		},
	})
}

// test the installation of the extension in minikube
func testDiscovery(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	log.Info().Msg("Starting testDiscovery")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := e2e.PollForTarget(ctx, e, extvm.TargetIDVM, func(target discovery_kit_api.Target) bool {
		return e2e.HasAttribute(target, "gcp-vm.name", "test")
	})
	// we do not have a real gcp vm running, so we expect an error
	require.Error(t, err)
}
