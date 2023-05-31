/*
Copyright 2023 The Kubernetes Authors.

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

package kubeletnotifier

import (
	"fmt"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/fsnotify/fsnotify"
)

type EventType string

const (
	IntervalBased EventType = "intervalBased"
	FSUpdate      EventType = "fsUpdate"

	devicePluginsDirName = "device-plugins"
)

var stateFiles = sets.NewString(
	"cpu_manager_state",
	"memory_manager_state",
	"kubelet_internal_checkpoint",
)

type Notifier struct {
	sleepInterval time.Duration
	// destination where notifications are sent
	dest    chan<- Info
	fsEvent <-chan fsnotify.Event
}

type Info struct {
	Event EventType
}

func New(sleepInterval time.Duration, dest chan<- Info, kubeletStateDir string) (*Notifier, error) {
	devicePluginsDir := path.Join(kubeletStateDir, devicePluginsDirName)
	ch, err := createFSWatcherEvent([]string{kubeletStateDir, devicePluginsDir})
	if err != nil {
		return nil, err
	}
	return &Notifier{
		sleepInterval: sleepInterval,
		dest:          dest,
		fsEvent:       ch,
	}, nil
}

func (n *Notifier) Run() {
	timeEvents := make(<-chan time.Time)
	if n.sleepInterval > 0 {
		ticker := time.NewTicker(n.sleepInterval)
		timeEvents = ticker.C
	}

	for {
		select {
		case <-timeEvents:
			klog.V(5).InfoS("timer update received")
			i := Info{Event: IntervalBased}
			n.dest <- i

		case e := <-n.fsEvent:
			basename := path.Base(e.Name)
			klog.V(5).InfoS("fsnotify event received", "filename", basename, "op", e.Op)
			if stateFiles.Has(basename) {
				i := Info{Event: FSUpdate}
				n.dest <- i
			}
		}
	}
}

func createFSWatcherEvent(fsWatchPaths []string) (chan fsnotify.Event, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, path := range fsWatchPaths {
		if err = fsWatcher.Add(path); err != nil {
			return nil, fmt.Errorf("failed to watch: %q; %w", path, err)
		}
	}
	return fsWatcher.Events, nil
}
