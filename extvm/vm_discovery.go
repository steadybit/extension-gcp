/*
* Copyright 2023 steadybit GmbH. All rights reserved.
*/

package extvm

import (
  compute "cloud.google.com/go/compute/apiv1"
  computepb "cloud.google.com/go/compute/apiv1/computepb"
  "context"
  "errors"
  "github.com/rs/zerolog/log"
  "github.com/steadybit/discovery-kit/go/discovery_kit_api"
  extension_kit "github.com/steadybit/extension-kit"
  "github.com/steadybit/extension-kit/extbuild"
  "github.com/steadybit/extension-kit/exthttp"
  "github.com/steadybit/extension-kit/extutil"
  "google.golang.org/api/iterator"
  "net/http"
)

const discoveryBasePath = "/" + TargetIDVM + "/discovery"

func RegisterDiscoveryHandlers() {
	exthttp.RegisterHttpHandler(discoveryBasePath, exthttp.GetterAsHandler(getDiscoveryDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/target-description", exthttp.GetterAsHandler(getTargetDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/attribute-descriptions", exthttp.GetterAsHandler(getAttributeDescriptions))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/discovered-targets", getDiscoveredVMs)
	exthttp.RegisterHttpHandler(discoveryBasePath+"/rules/gcp-vm-to-container", exthttp.GetterAsHandler(getToContainerEnrichmentRule))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/rules/gcp-vm-to-host", exthttp.GetterAsHandler(getToHostEnrichmentRule))
}

var (
	virtualMachinesClient *compute.InstancesClient
)

func GetDiscoveryList() discovery_kit_api.DiscoveryList {
	return discovery_kit_api.DiscoveryList{
		Discoveries: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath,
			},
		},
		TargetTypes: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/target-description",
			},
		},
		TargetAttributes: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/attribute-descriptions",
			},
		},
		TargetEnrichmentRules: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/rules/gcp-vm-to-host",
			},
     {
				Method: "GET",
				Path:   discoveryBasePath + "/rules/gcp-vm-to-container",
			},
		},
	}
}

func getDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         TargetIDVM,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         discoveryBasePath + "/discovered-targets",
			CallInterval: extutil.Ptr("1m"),
		},
	}
}

func getTargetDescription() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:      TargetIDVM,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),

		// Labels used in the UI
		Label: discovery_kit_api.PluralLabel{One: "Google Cloud Virtual Machine", Other: "Google Cloud Virtual Machines"},

		// Category for the targets to appear in
		Category: extutil.Ptr("cloud"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "gcp.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "steadybit.label",
					Direction: "ASC",
				},
			},
		},
	}
}

func getAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
			{
				Attribute: "gcp-vm.hostname",
				Label: discovery_kit_api.PluralLabel{
					One:   "Host name",
					Other: "Host names",
				},
			},
			{
				Attribute: "gcp.location",
				Label: discovery_kit_api.PluralLabel{
					One:   "Location",
					Other: "Locations",
				},
			},
			{
				Attribute: "gcp-vm.network.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "Network ID",
					Other: "Network IDs",
				},
			},
			{
				Attribute: "gcp-vm.os.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS name",
					Other: "OS names",
				},
			},
			{
				Attribute: "gcp-vm.os.type",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS type",
					Other: "OS types",
				},
			},
			{
				Attribute: "gcp-vm.os.version",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS version",
					Other: "OS versions",
				},
			},
			{
				Attribute: "gcp-vm.power.state",
				Label: discovery_kit_api.PluralLabel{
					One:   "Power state",
					Other: "Power states",
				},
			},
			{
				Attribute: "gcp-vm.tags",
				Label: discovery_kit_api.PluralLabel{
					One:   "Tags",
					Other: "Tags",
				},
			},
			{
				Attribute: "gcp-vm.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM ID",
					Other: "VM IDs",
				},
			},
			{
				Attribute: "gcp-vm.size",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM size",
					Other: "VM sizes",
				},
			},
			{
				Attribute: "gcp-vm.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM name",
					Other: "VM names",
				},
			},
			{
				Attribute: "gcp.project.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "Project ID",
					Other: "Project IDs",
				},
			},
      {
				Attribute: "gcp.project.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "Project Name",
					Other: "Project Names",
				},
			},
		},
	}
}



func getDiscoveredVMs(w http.ResponseWriter, _ *http.Request, _ []byte) {
	ctx := context.Background()
  instancesClient, err := GetGcpInstancesClient(ctx)
	if err != nil {
		log.Error().Msgf("failed to get client: %v", err)
		return
	}
  defer instancesClient.Close()
 targets, err := GetAllVirtualMachines(ctx, instancesClient)
	if err != nil {
		log.Error().Msgf("failed to get all virtual machines: %v", err)
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect gcp virtual machines information", err))
		return
	}

 exthttp.WriteBody(w, discovery_kit_api.DiscoveryData{Targets: &targets})
}

//type GCPInstancesApi interface {
//  AggregatedList(ctx context.Context, req *computepb.AggregatedListInstancesRequest, opts ...gax.CallOption) *compute.InstancesScopedListPairIterator
//}


func GetAllVirtualMachines(ctx context.Context, client *compute.InstancesClient) ([]discovery_kit_api.Target, error) {
  projectID := "extension-gcp" //TODO: get project id from config
  //Use the `MaxResults` parameter to limit the number of results that the API returns per response page.
  req := &computepb.AggregatedListInstancesRequest{
   Project:    projectID,
  }
  it := client.AggregatedList(ctx, req)


  for {
    pair, err := it.Next()
    if errors.Is(err, iterator.Done) {
      break
    }
    if err != nil {
      log.Error().Msgf("failed to iterate through instances: %v", err)
      return nil, err
    }
    instances := pair.Value.Instances
    if len(instances) > 0 {
      log.Info().Msgf("Instances for %s", pair.Key)
      for _, instance := range instances {
        log.Info().Msgf("- %s %s\n", instance.GetName(), instance.GetMachineType())
      }
    }
  }

   targets := make([]discovery_kit_api.Target, 0)
		//// Print the obtained query results
		//log.Debug().Msgf("Virtual Machines found: " + strconv.FormatInt(*results.TotalRecords, 10))
		//	for _, r := range m {
		//		items := r.(map[string]interface{})
		//		attributes := make(map[string][]string)
    //
		//		//attributes["gcp-vm.vm.name"] = []string{items["name"].(string)}
		//		//attributes["gcp.subscription.id"] = []string{items["subscriptionId"].(string)}
		//		//attributes["gcp-vm.vm.id"] = []string{getPropertyValue(properties, "vmId")}
		//		//attributes["gcp-vm.vm.size"] = []string{getPropertyValue(hardwareProfile, "vmSize")}
		//		//attributes["gcp-vm.os.name"] = []string{getPropertyValue(instanceView, "osName")}
		//		//attributes["gcp-vm.hostname"] = []string{getPropertyValue(instanceView, "computerName")}
		//		//attributes["gcp-vm.os.version"] = []string{getPropertyValue(instanceView, "osVersion")}
		//		//attributes["gcp-vm.os.type"] = []string{getPropertyValue(osDisk, "osType")}
		//		//attributes["gcp-vm.power.state"] = []string{getPropertyValue(powerState, "code")}
		//		//attributes["gcp-vm.network.id"] = []string{getPropertyValue(networkInterfaces, "id")}
		//		//attributes["gcp.location"] = []string{getPropertyValue(items, "location")}
		//		//attributes["gcp.resource-group.name"] = []string{getPropertyValue(items, "resourceGroup")}
    //
		//		for k, v := range common.GetMapValue(items, "tags") {
		//			attributes[fmt.Sprintf("gcp-vm.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
		//		}
    //
		//		targets = append(targets, discovery_kit_api.Target{
		//			Id:         " ",
		//			TargetType: TargetIDVM,
		//			Label:      items["name"].(string),
		//			Attributes: attributes,
		//		})
		//	}
	  return targets, nil
}

func getToHostEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_gcp.gcp-vm-to-host",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"gcp-vm.hostname": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_host.host",
			Selector: map[string]string{
				"host.hostname": "${src.gcp-vm.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.subscription.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.vm.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.network.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.location",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.resource-group.name",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.label.",
			},
		},
	}
}

func getToContainerEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_gcp.gcp-vm-to-container",
		Version: extbuild.GetSemverVersionStringOrUnknown(),

		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"gcp-vm.hostname": "${dest.container.host}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_container.container",
			Selector: map[string]string{
				"container.host": "${src.gcp-vm.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.subscription.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.vm.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp-vm.network.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.location",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "gcp.resource-group.name",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "gcp-vm.label.",
			},
		},
	}
}

