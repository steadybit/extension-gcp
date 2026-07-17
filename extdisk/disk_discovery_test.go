/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extdisk

import (
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/stretchr/testify/assert"
)

func TestClassifyScope(t *testing.T) {
	z, r := classifyScope("zones/europe-west1-a")
	assert.Equal(t, "europe-west1-a", z)
	assert.Equal(t, "", r)

	z, r = classifyScope("regions/europe-west1")
	assert.Equal(t, "", z)
	assert.Equal(t, "europe-west1", r)

	z, r = classifyScope("global")
	assert.Equal(t, "", z)
	assert.Equal(t, "", r)
}

func TestToDiskTarget_Populated(t *testing.T) {
	disk := &computepb.Disk{
		Name:                   ptr("my-disk"),
		SelfLink:               ptr("https://www.googleapis.com/compute/v1/projects/proj-a/zones/europe-west1-a/disks/my-disk"),
		Type:                   ptr("https://www.googleapis.com/compute/v1/projects/proj-a/zones/europe-west1-a/diskTypes/pd-ssd"),
		SizeGb:                 ptrInt64(100),
		Status:                 ptr("READY"),
		Users:                  []string{"z-user", "a-user"},
		SourceImage:            ptr("projects/img/global/images/family/x"),
		SourceSnapshot:         ptr("projects/snap"),
		DiskEncryptionKey:      &computepb.CustomerEncryptionKey{KmsKeyName: ptr("projects/kms/keys/x")},
		PhysicalBlockSizeBytes: ptrInt64(4096),
		ProvisionedIops:        ptrInt64(3000),
		ProvisionedThroughput:  ptrInt64(200),
		Architecture:           ptr("X86_64"),
		Labels:                 map[string]string{"team": "core"},
	}
	target := toDiskTarget(disk, "europe-west1-a", "", "proj-a")

	assert.Equal(t, TargetIDDisk, target.TargetType)
	assert.Equal(t, "my-disk", target.Label)
	assert.Equal(t, "https://www.googleapis.com/compute/v1/projects/proj-a/zones/europe-west1-a/disks/my-disk", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"my-disk"}, target.Attributes["gcp.persistent-disk.name"])
	assert.Equal(t, []string{"europe-west1-a"}, target.Attributes[attrZone])
	assert.Equal(t, []string{"pd-ssd"}, target.Attributes[attrType])
	assert.Equal(t, []string{"100"}, target.Attributes[attrSizeGB])
	assert.Equal(t, []string{"READY"}, target.Attributes["gcp.persistent-disk.status"])
	assert.Equal(t, []string{"a-user", "z-user"}, target.Attributes["gcp.persistent-disk.users"])
	assert.Equal(t, []string{"projects/img/global/images/family/x"}, target.Attributes["gcp.persistent-disk.source-image"])
	assert.Equal(t, []string{"projects/snap"}, target.Attributes["gcp.persistent-disk.source-snapshot"])
	assert.Equal(t, []string{"projects/kms/keys/x"}, target.Attributes["gcp.persistent-disk.kms-key-name"])
	assert.Equal(t, []string{"4096"}, target.Attributes["gcp.persistent-disk.physical-block-size-bytes"])
	assert.Equal(t, []string{"3000"}, target.Attributes["gcp.persistent-disk.provisioned-iops"])
	assert.Equal(t, []string{"200"}, target.Attributes["gcp.persistent-disk.provisioned-throughput"])
	assert.Equal(t, []string{"X86_64"}, target.Attributes["gcp.persistent-disk.architecture"])
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.persistent-disk.label.team"])
}

func TestToDiskTarget_RegionalNoTypeUrl(t *testing.T) {
	disk := &computepb.Disk{
		Name: ptr("regional"),
		Type: ptr("pd-standard"), // no slash in URL
	}
	target := toDiskTarget(disk, "", "europe-west1", "proj-a")

	assert.NotContains(t, target.Attributes, attrZone)
	assert.Equal(t, []string{"europe-west1"}, target.Attributes["gcp.persistent-disk.region"])
	assert.Equal(t, []string{"pd-standard"}, target.Attributes[attrType])
}

func TestToDiskTarget_Sparse(t *testing.T) {
	disk := &computepb.Disk{
		Name: ptr("sparse"),
	}
	target := toDiskTarget(disk, "", "", "proj-a")

	assert.Equal(t, "sparse", target.Label)
	assert.NotContains(t, target.Attributes, attrZone)
	assert.NotContains(t, target.Attributes, "gcp.persistent-disk.region")
	assert.NotContains(t, target.Attributes, attrType)
	assert.NotContains(t, target.Attributes, attrSizeGB)
	assert.NotContains(t, target.Attributes, "gcp.persistent-disk.status")
	assert.NotContains(t, target.Attributes, "gcp.persistent-disk.users")
	assert.NotContains(t, target.Attributes, "gcp.persistent-disk.kms-key-name")
}

func TestDescribeMethods(t *testing.T) {
	d := &diskDiscovery{}
	assert.Equal(t, TargetIDDisk, d.Describe().Id)
	assert.Equal(t, TargetIDDisk, d.DescribeTarget().Id)
	assert.NotEmpty(t, d.DescribeAttributes())
}

func TestNewDiskDiscovery(t *testing.T) {
	assert.NotNil(t, NewDiskDiscovery())
}

func ptr[T any](v T) *T { return &v }
func ptrInt64(v int64) *int64 {
	return &v
}
