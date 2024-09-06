/*
Copyright 2024 The Kubernetes Authors.

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

package compat

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"

	pkgcompat "sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat"
)

var (
	image string
)

// TODO:
// * add secrets handling
// * add validation strategy

var validateNodeCmd = &cobra.Command{
	Use:   "validate-node",
	Short: "Validate node based on image compatibility metadata",
	Long:  "Validate node based on image compatibility metadata from the NFD artifact",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		ref, err := registry.ParseReference(image)
		if err != nil {
			return err
		}
		results, err := pkgcompat.ValidateNode(ctx, &ref)
		if err != nil {
			return err
		}
		// TODO: add a better report
		for i, r := range results {
			msg := fmt.Sprintf("COMPATIBILITY GROUP NO %d", i+1)
			if r.IsNodeValid {
				msg += " \033[32mSUCEEDS\033[0m"
			} else {
				msg += " \033[31mFAILS\033[0m"
			}
			msg += fmt.Sprintf(" (%d/%d)", r.ChecksSucceded, r.ChecksTotal)
			fmt.Println(msg)
			fmt.Println("========================")
			for k, v := range r.LabelsCheck {
				fmt.Printf("%s - %s\n", k, v)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	CompatCmd.AddCommand(validateNodeCmd)
	validateNodeCmd.Flags().StringVar(&image, "image", "", "URL of image with compatibility metadata")
	if err := validateNodeCmd.MarkFlagRequired("image"); err != nil {
		panic(err)
	}
}
