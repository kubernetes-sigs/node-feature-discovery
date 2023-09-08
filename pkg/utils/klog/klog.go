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

package klog

import (
	"flag"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

// KlogConfigOpts defines klog configuration options
type KlogConfigOpts map[string]string

// InitKlogFlags function is responsible for initializing klog flags.
func InitKlogFlags(flagset *flag.FlagSet) map[string]*utils.KlogFlagVal {
	klogFlags := make(map[string]*utils.KlogFlagVal)

	flags := flag.NewFlagSet("klog flags", flag.ContinueOnError)
	klog.InitFlags(flags)
	flags.VisitAll(func(f *flag.Flag) {
		name := klogConfigOptName(f.Name)
		klogFlags[name] = utils.NewKlogFlagVal(f)
		flagset.Var(klogFlags[name], f.Name, f.Usage)
	})

	return klogFlags
}

// MergeKlogConfiguration merges klog command line flags to klog configuration file options
func MergeKlogConfiguration(klogArgs map[string]*utils.KlogFlagVal, klogConfig KlogConfigOpts) error {
	for k, a := range klogArgs {
		if !a.IsSetFromCmdline() {
			v, ok := klogConfig[k]
			if !ok {
				v = a.DefValue()
			}
			if err := a.SetFromConfig(v); err != nil {
				return fmt.Errorf("failed to set logger option klog.%s = %v: %v", k, v, err)
			}
		}
	}
	for k := range klogConfig {
		if _, ok := klogArgs[k]; !ok {
			klog.InfoS("unknown logger option in config", "optionName", k)
		}
	}

	return nil
}

func klogConfigOptName(flagName string) string {
	split := strings.Split(flagName, "_")
	for i, v := range split[1:] {
		split[i+1] = strings.ToUpper(v[0:1]) + v[1:]
	}
	return strings.Join(split, "")
}
