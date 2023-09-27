/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package main

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/event-kit/go/event_kit_api"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-gcp/extvm"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthealth"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	_ "net/http/pprof" //allow pprof
)

func main() {
	// Most Steadybit extensions leverage zerolog. To encourage persistent logging setups across extensions,
	// you may leverage the extlogging package to initialize zerolog. Among others, this package supports
	// configuration of active log levels and the log format (JSON or plain text).
	//
	// Example
	//  - to activate JSON logging, set the environment variable STEADYBIT_LOG_FORMAT="json"
	//  - to set the log level to debug, set the environment variable STEADYBIT_LOG_LEVEL="debug"
	extlogging.InitZeroLog()

	// Build information is set at compile-time. This line writes the build information to the log.
	// The information is mostly handy for debugging purposes.
	extbuild.PrintBuildInformation()

	//This will start /health/liveness and /health/readiness endpoints on port 8081 for use with kubernetes
	//The port can be configured using the STEADYBIT_EXTENSION_HEALTH_PORT environment variable
	exthealth.SetReady(false)
	exthealth.StartProbes(8081)

	// Most extensions require some form of configuration. These calls exist to parse and validate the
	// configuration obtained from environment variables.
	config.ParseConfiguration()
	config.ValidateConfiguration()

	// This call registers a handler for the extension's root path. This is the path initially accessed
	// by the Steadybit agent to obtain the extension's capabilities.
	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))

	// This is a section you will most likely want to change: The registration of HTTP handlers
	// for your extension. You might want to change these because the names do not fit, or because
	// you do not have a need for all of them.
	extvm.RegisterDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extvm.NewVirtualMachineStateAction())

	//This will install a signal handlder, that will stop active actions when receiving a SIGURS1, SIGTERM or SIGINT
	action_kit_sdk.InstallSignalHandler()

	//This will register the coverage endpoints for the extension (used by action_kit_test)
	action_kit_sdk.RegisterCoverageEndpoints()

	//This will switch the readiness state of the application to true.
	exthealth.SetReady(true)

	exthttp.Listen(exthttp.ListenOpts{
		// This is the default port under which your extension is accessible.
		// The port can be configured externally through the
		// STEADYBIT_EXTENSION_PORT environment variable.
		// We suggest that you keep port 8092 as the default.
		Port: 8093,
	})
}

// ExtensionListResponse exists to merge the possible root path responses supported by the
// various extension kits. In this case, the response for ActionKit, DiscoveryKit and EventKit.
type ExtensionListResponse struct {
	action_kit_api.ActionList       `json:",inline"`
	discovery_kit_api.DiscoveryList `json:",inline"`
	event_kit_api.EventListenerList `json:",inline"`
}

func getExtensionList() ExtensionListResponse {
	discoveries := make([]discovery_kit_api.DescribingEndpointReference, 0)
	discoveries = append(discoveries, extvm.GetDiscoveryList().Discoveries...)
	targetAttributes := make([]discovery_kit_api.DescribingEndpointReference, 0)
	targetAttributes = append(targetAttributes, extvm.GetDiscoveryList().TargetAttributes...)
	targetEnrichmentRukles := make([]discovery_kit_api.DescribingEndpointReference, 0)
	targetEnrichmentRukles = append(targetEnrichmentRukles, extvm.GetDiscoveryList().TargetEnrichmentRules...)
	targetTypes := make([]discovery_kit_api.DescribingEndpointReference, 0)
	targetTypes = append(targetTypes, extvm.GetDiscoveryList().TargetTypes...)

	return ExtensionListResponse{
		ActionList: action_kit_sdk.GetActionList(),

		DiscoveryList: discovery_kit_api.DiscoveryList{
			Discoveries:           discoveries,
			TargetAttributes:      targetAttributes,
			TargetEnrichmentRules: targetEnrichmentRukles,
			TargetTypes:           targetTypes,
		},
	}
}
