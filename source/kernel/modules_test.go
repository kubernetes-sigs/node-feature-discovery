/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBuiltinModules(t *testing.T) {
	content := `kernel/drivers/md/dm-mod.ko
kernel/drivers/usb/host/xhci-hcd.ko
kernel/drivers/nvme/host/nvme-core.ko
kernel/net/8021q/8021q.ko
kernel/fs/ext4/ext4.ko

  kernel/drivers/scsi/scsi_mod.ko  
kernel/drivers/not-a-module.o
`
	assert.Equal(t,
		[]string{"dm_mod", "xhci_hcd", "nvme_core", "8021q", "ext4", "scsi_mod"},
		parseBuiltinModules(content))

	assert.Empty(t, parseBuiltinModules(""))
}
