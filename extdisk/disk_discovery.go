/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extdisk

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-gcp/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/iterator"
)

const (
	TargetIDDisk = "com.steadybit.extension_gcp.persistent-disk"
	targetIcon   = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgM2M0Ljk3IDAgOSAxLjM0IDkgM3YxMmMwIDEuNjYtNC4wMyAzLTkgM3MtOS0xLjM0LTktM1Y2YzAtMS42NiA0LjAzLTMgOS0zem0wIDJjLTMuOSAwLTcgLjg5LTcgMnMzLjEgMiA3IDIgNy0uODkgNy0yLTMuMS0yLTctMnoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPjwvc3ZnPg=="

	// Attribute names extracted per Sonar go:S1192.
	attrType      = "gcp.persistent-disk.type"
	attrSizeGB    = "gcp.persistent-disk.size-gb"
	attrZone      = "gcp.persistent-disk.zone"
	attrProjectID = "gcp.project.id"
)

type diskDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*diskDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*diskDiscovery)(nil)
)

func NewDiskDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&diskDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *diskDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDDisk,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *diskDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDDisk,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Persistent Disk", Other: "Persistent Disks"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: attrType},
				{Attribute: attrSizeGB},
				{Attribute: attrZone},
				{Attribute: attrProjectID},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *diskDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.persistent-disk.name", Label: discovery_kit_api.PluralLabel{One: "Disk name", Other: "Disk names"}},
		{Attribute: attrType, Label: discovery_kit_api.PluralLabel{One: "Disk type", Other: "Disk types"}},
		{Attribute: attrSizeGB, Label: discovery_kit_api.PluralLabel{One: "Disk size (GiB)", Other: "Disk sizes (GiB)"}},
		{Attribute: attrZone, Label: discovery_kit_api.PluralLabel{One: "Disk zone", Other: "Disk zones"}},
		{Attribute: "gcp.persistent-disk.region", Label: discovery_kit_api.PluralLabel{One: "Disk region", Other: "Disk regions"}},
		{Attribute: "gcp.persistent-disk.status", Label: discovery_kit_api.PluralLabel{One: "Disk status", Other: "Disk statuses"}},
		{Attribute: "gcp.persistent-disk.users", Label: discovery_kit_api.PluralLabel{One: "Disk attached VM", Other: "Disk attached VMs"}},
		{Attribute: "gcp.persistent-disk.source-image", Label: discovery_kit_api.PluralLabel{One: "Disk source image", Other: "Disk source images"}},
		{Attribute: "gcp.persistent-disk.source-snapshot", Label: discovery_kit_api.PluralLabel{One: "Disk source snapshot", Other: "Disk source snapshots"}},
		{Attribute: "gcp.persistent-disk.kms-key-name", Label: discovery_kit_api.PluralLabel{One: "Disk KMS key name", Other: "Disk KMS key names"}},
		{Attribute: "gcp.persistent-disk.physical-block-size-bytes", Label: discovery_kit_api.PluralLabel{One: "Disk physical block size", Other: "Disk physical block sizes"}},
		{Attribute: "gcp.persistent-disk.provisioned-iops", Label: discovery_kit_api.PluralLabel{One: "Disk provisioned IOPS", Other: "Disk provisioned IOPS"}},
		{Attribute: "gcp.persistent-disk.provisioned-throughput", Label: discovery_kit_api.PluralLabel{One: "Disk provisioned throughput", Other: "Disk provisioned throughputs"}},
		{Attribute: "gcp.persistent-disk.architecture", Label: discovery_kit_api.PluralLabel{One: "Disk architecture", Other: "Disk architectures"}},
		{Attribute: attrProjectID, Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *diskDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := compute.NewDisksRESTClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Disks client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllDisks(ctx, client, access.ProjectID)
	}, ctx, "persistent-disk")
}

func getAllDisks(ctx context.Context, client *compute.DisksClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	it := client.AggregatedList(ctx, &computepb.AggregatedListDisksRequest{Project: projectID})
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to aggregate-list disks")
			return nil, err
		}
		if pair.Value == nil {
			continue
		}
		zone, region := classifyScope(pair.Key)
		for _, disk := range pair.Value.Disks {
			targets = append(targets, toDiskTarget(disk, zone, region, projectID))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesPersistentDisk), nil
}

func classifyScope(key string) (zone, region string) {
	switch {
	case strings.HasPrefix(key, "zones/"):
		return strings.TrimPrefix(key, "zones/"), ""
	case strings.HasPrefix(key, "regions/"):
		return "", strings.TrimPrefix(key, "regions/")
	}
	return "", ""
}

func toDiskTarget(disk *computepb.Disk, zone, region, projectID string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes[attrProjectID] = []string{projectID}
	attributes["gcp.persistent-disk.name"] = []string{disk.GetName()}
	utils.SetStr(attributes, attrZone, zone)
	utils.SetStr(attributes, "gcp.persistent-disk.region", region)
	utils.SetStr(attributes, attrType, shortDiskType(disk.GetType()))
	utils.SetInt64IfPositive(attributes, attrSizeGB, disk.GetSizeGb())
	utils.SetStr(attributes, "gcp.persistent-disk.status", disk.GetStatus())
	addDiskUsersAttr(attributes, disk.GetUsers())
	utils.SetStr(attributes, "gcp.persistent-disk.source-image", disk.GetSourceImage())
	utils.SetStr(attributes, "gcp.persistent-disk.source-snapshot", disk.GetSourceSnapshot())
	addDiskEncryptionAttrs(attributes, disk.GetDiskEncryptionKey())
	utils.SetInt64IfPositive(attributes, "gcp.persistent-disk.physical-block-size-bytes", disk.GetPhysicalBlockSizeBytes())
	utils.SetInt64IfPositive(attributes, "gcp.persistent-disk.provisioned-iops", disk.GetProvisionedIops())
	utils.SetInt64IfPositive(attributes, "gcp.persistent-disk.provisioned-throughput", disk.GetProvisionedThroughput())
	utils.SetStr(attributes, "gcp.persistent-disk.architecture", disk.GetArchitecture())
	for k, v := range disk.GetLabels() {
		utils.SetStr(attributes, fmt.Sprintf("gcp.persistent-disk.label.%s", strings.ToLower(k)), v)
	}

	return discovery_kit_api.Target{
		Id:         disk.GetSelfLink(),
		TargetType: TargetIDDisk,
		Label:      disk.GetName(),
		Attributes: attributes,
	}
}

// shortDiskType returns the last path component of a disk type URL; the raw
// value is returned unchanged when it has no "/" separator or is empty.
func shortDiskType(t string) string {
	if t == "" {
		return ""
	}
	if i := strings.LastIndex(t, "/"); i >= 0 {
		return t[i+1:]
	}
	return t
}

func addDiskUsersAttr(attrs map[string][]string, users []string) {
	if len(users) == 0 {
		return
	}
	sorted := append([]string(nil), users...)
	sort.Strings(sorted)
	attrs["gcp.persistent-disk.users"] = sorted
}

func addDiskEncryptionAttrs(attrs map[string][]string, dek *computepb.CustomerEncryptionKey) {
	if dek == nil {
		return
	}
	utils.SetStr(attrs, "gcp.persistent-disk.kms-key-name", dek.GetKmsKeyName())
}
