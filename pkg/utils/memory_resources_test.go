/*
Copyright 2021 The Kubernetes Authors.

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

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

const (
	HugepageSize2Mi = 2048
	HugepageSize1Gi = 1048576
)

const testMeminfo = `Node 0 MemTotal:       32718644 kB
Node 0 MemFree:         2915988 kB
Node 0 MemUsed:        29802656 kB
Node 0 Active:         19631832 kB
Node 0 Inactive:        8089096 kB
Node 0 Active(anon):   10104396 kB
Node 0 Inactive(anon):   511432 kB
Node 0 Active(file):    9527436 kB
Node 0 Inactive(file):  7577664 kB
Node 0 Unevictable:      637864 kB
Node 0 Mlocked:               0 kB
Node 0 Dirty:              1140 kB
Node 0 Writeback:             0 kB
Node 0 FilePages:      18206092 kB
Node 0 Mapped:          2000244 kB
Node 0 AnonPages:      10152780 kB
Node 0 Shmem:           1249348 kB
Node 0 KernelStack:       37440 kB
Node 0 PageTables:       110460 kB
Node 0 NFS_Unstable:          0 kB
Node 0 Bounce:                0 kB
Node 0 WritebackTmp:          0 kB
Node 0 KReclaimable:     843624 kB
Node 0 Slab:            1198060 kB
Node 0 SReclaimable:     843624 kB
Node 0 SUnreclaim:       354436 kB
Node 0 AnonHugePages:     26624 kB
Node 0 ShmemHugePages:        0 kB
Node 0 ShmemPmdMapped:        0 kB
Node 0 FileHugePages:        0 kB
Node 0 FilePmdMapped:        0 kB
Node 0 HugePages_Total:     0
Node 0 HugePages_Free:      0
Node 0 HugePages_Surp:      0`

func TestGetMemoryResourceCounters(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "fakehp")
	if err != nil {
		t.Errorf("failed to create temporary directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(rootDir); err != nil {
			t.Logf("failed to remove temporary directory %q: %v", rootDir, err)
		}
	}()

	sysBusNodeBasepath = rootDir

	// set mock hugepages
	if err := makeHugepagesTree(rootDir, 2); err != nil {
		t.Errorf("failed to setup the fake tree on %q: %v", rootDir, err)
	}
	if err := setHPCount(rootDir, 0, HugepageSize2Mi, 6); err != nil {
		t.Errorf("failed to setup hugepages on node %d the fake tree on %q: %v", 0, rootDir, err)
	}
	if err := setHPCount(rootDir, 1, HugepageSize2Mi, 8); err != nil {
		t.Errorf("failed to setup hugepages on node %d the fake tree on %q: %v", 0, rootDir, err)
	}

	// set mock memory
	if err := makeMemoryTree(rootDir, 2); err != nil {
		t.Errorf("failed to setup the fake tree on %q: %v", rootDir, err)
	}

	memoryCounters, err := GetNumaMemoryResources()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if memoryCounters[0]["hugepages-2Mi"] != 12582912 {
		t.Errorf("found unexpected amount of 2Mi hugepages under the NUMA node 0: %d", memoryCounters[0]["hugepages-2Mi"])
	}
	if memoryCounters[1]["hugepages-2Mi"] != 16777216 {
		t.Errorf("found unexpected amount of 2Mi hugepages under the NUMA node 1: %d", memoryCounters[1]["hugepages-2Mi"])
	}

	if memoryCounters[0]["hugepages-1Gi"] != 0 {
		t.Errorf("found unexpected 1Gi hugepages for node 0: %v", memoryCounters[0]["hugepages-1Gi"])
	}
	if memoryCounters[1]["hugepages-1Gi"] != 0 {
		t.Errorf("found unexpected 1Gi hugepages for node 1: %v", memoryCounters[0]["hugepages-1Gi"])
	}

	if memoryCounters[0]["memory"] != 32718644*1024 {
		t.Errorf("found unexpected amount of memory under the NUMA node 0: %d", memoryCounters[0][corev1.ResourceMemory])
	}

	if memoryCounters[1]["memory"] != 32718644*1024 {
		t.Errorf("found unexpected amount of memory under the NUMA node 1: %d", memoryCounters[0][corev1.ResourceMemory])
	}
}

func makeMemoryTree(root string, numNodes int) error {
	for idx := range numNodes {
		path := filepath.Join(
			root,
			fmt.Sprintf("node%d", idx),
		)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
		meminfoFile := filepath.Join(path, "meminfo")
		if err := os.WriteFile(meminfoFile, []byte(testMeminfo), 0644); err != nil {
			return err
		}

	}
	return nil
}

func makeHugepagesTree(root string, numNodes int) error {
	for idx := range numNodes {
		for _, size := range []int{HugepageSize2Mi, HugepageSize1Gi} {
			path := filepath.Join(
				root,
				fmt.Sprintf("node%d", idx),
				"hugepages",
				fmt.Sprintf("hugepages-%dkB", size),
			)
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			if err := setHPCount(root, idx, size, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

func setHPCount(root string, nodeID, pageSize, numPages int) error {
	path := filepath.Join(
		root,
		fmt.Sprintf("node%d", nodeID),
		"hugepages",
		fmt.Sprintf("hugepages-%dkB", pageSize),
		"nr_hugepages",
	)
	return os.WriteFile(path, fmt.Appendf(nil, "%d", numPages), 0644)
}
