// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
  "github.com/steadybit/action-kit/go/action_kit_api/v2"
  "github.com/steadybit/action-kit/go/action_kit_test/e2e"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-scaffold",
		Port: 8080,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{"--set", "logging.level=debug"}
		},
	}

	mOpts := e2e.DefaultMiniKubeOpts
	mOpts.Runtimes = []e2e.Runtime{e2e.RuntimeDocker}

	e2e.WithMinikube(t, mOpts, &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "run scaffold",
			Test: testRunscaffold,
		},
	})
}

func testRunscaffold(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	config := struct{}{}
	exec, err := e.RunAction("com.steadybit.extension_scaffold.robot.log", &action_kit_api.Target{
    Name: "robot",
  }, config, nil)
	require.NoError(t, err)
	e2e.AssertLogContains(t, m, e.Pod, "Logging in log action **start**")
	require.NoError(t, exec.Cancel())
}
