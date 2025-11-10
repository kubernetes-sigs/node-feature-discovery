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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"

	"sigs.k8s.io/node-feature-discovery/cmd/nfd/subcmd/compat/options"
	artifactcli "sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/artifact-client"
	nodevalidator "sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/node-validator"
	"sigs.k8s.io/node-feature-discovery/source"
)

var (
	image      string
	tags       []string
	platform   options.PlatformOption
	plainHTTP  bool
	outputJSON bool

	// secrets
	readPassword    bool
	readAccessToken bool
	username        string
	password        string
	accessToken     string

	validateExample = `
# Validate image compatibility
nfd compat validate-node --image <image-url>`
)

var validateNodeCmd = &cobra.Command{
	Use:     "validate-node",
	Short:   "Perform node validation based on its associated image compatibility artifact",
	Example: validateExample,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		if err = platform.Parse(cmd); err != nil {
			return err
		}

		if readAccessToken && readPassword {
			return fmt.Errorf("cannot use --registry-token-stdin and --registry-password-stdin at the same time")
		} else if readAccessToken {
			accessToken, err = readStdin()
			if err != nil {
				return err
			}
		} else if readPassword {
			password, err = readStdin()
			if err != nil {
				return err
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		ref, err := registry.ParseReference(image)
		if err != nil {
			return err
		}

		sources := map[string]source.FeatureSource{}
		for k, v := range source.GetAllFeatureSources() {
			if ts, ok := v.(source.SupplementalSource); ok && ts.DisableByDefault() {
				continue
			}
			sources[k] = v
		}

		authOpt := artifactcli.WithAuthDefault()
		if username != "" && password != "" {
			authOpt = artifactcli.WithAuthPassword(username, password)
		} else if accessToken != "" {
			authOpt = artifactcli.WithAuthToken(accessToken)
		}

		ac := artifactcli.New(
			&ref,
			artifactcli.WithArgs(artifactcli.Args{PlainHttp: plainHTTP}),
			artifactcli.WithPlatform(platform.Platform),
			authOpt,
		)

		nv := nodevalidator.New(
			nodevalidator.WithArgs(&nodevalidator.Args{Tags: tags}),
			nodevalidator.WithArtifactClient(ac),
			nodevalidator.WithSources(sources),
		)

		out, err := nv.Execute(ctx)
		if err != nil {
			return err
		}
		if outputJSON {
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			fmt.Printf("%s", b)
		} else {
			pprintResult(out)
		}

		return nil
	},
}

func readStdin() (string, error) {
	secretRaw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSuffix(string(secretRaw), "\n")
	secret = strings.TrimSuffix(secret, "\r")

	return secret, nil
}

func pprintResult(css []*nodevalidator.CompatibilityStatus) {
	for i, cs := range css {
		fmt.Print(text.Colors{text.FgCyan, text.Bold}.Sprintf("COMPATIBILITY SET #%d ", i+1))
		fmt.Print(text.FgCyan.Sprintf("Weight: %d", cs.Weight))
		if cs.Tag != "" {
			fmt.Print(text.FgCyan.Sprintf("; Tag: %s", cs.Tag))
		}
		fmt.Println()
		fmt.Println(text.FgWhite.Sprintf("Description: %s", cs.Description))

		for _, r := range cs.Rules {
			printTable(r)
		}
		fmt.Println()
	}
}

func printTable(rs nodevalidator.ProcessedRuleStatus) {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.SetOutputMirror(os.Stdout)
	t.Style().Format.Header = text.FormatDefault
	t.SetAutoIndex(true)

	validTxt := text.BgRed.Sprint(" FAIL ")
	if rs.IsMatch {
		validTxt = text.BgGreen.Sprint(" OK ")
	}
	ruleTxt := strings.ToUpper(fmt.Sprintf("rule: %s", rs.Name))

	t.SetTitle(text.Bold.Sprintf("%s - %s", ruleTxt, validTxt))
	t.AppendHeader(table.Row{"Feature", "Expression", "Matcher Type", "Status"})

	if mf := rs.MatchedExpressions; len(mf) > 0 {
		renderMatchFeatures(t, mf)
	}
	if ma := rs.MatchedAny; len(ma) > 0 {
		for _, elem := range ma {
			t.AppendSeparator()
			renderMatchFeatures(t, elem.MatchedExpressions)
		}
	}
	t.Render()
}

func renderMatchFeatures(t table.Writer, matchedExpressions []nodevalidator.MatchedExpression) {
	for _, fm := range matchedExpressions {
		fullFeatureDomain := fm.Feature
		if fm.Name != "" {
			fullFeatureDomain = fmt.Sprintf("%s.%s", fm.Feature, fm.Name)
		}

		addTableRows(t, fullFeatureDomain, fm.Expression.String(), fm.MatcherType, fm.IsMatch)
	}
}

func addTableRows(t table.Writer, fullFeatureDomain, expression string, matcherType nodevalidator.MatcherType, isMatch bool) {
	status := text.FgHiRed.Sprint("FAIL")
	if isMatch {
		status = text.FgHiGreen.Sprint("OK")
	}
	t.AppendRow(table.Row{fullFeatureDomain, expression, matcherType, status})
}

func init() {
	CompatCmd.AddCommand(validateNodeCmd)
	validateNodeCmd.Flags().StringVar(&image, "image", "", "the URL of the image containing compatibility metadata")
	validateNodeCmd.Flags().StringSliceVar(&tags, "tags", []string{}, "a list of tags that must match the tags set on the compatibility objects")
	validateNodeCmd.Flags().StringVar(&platform.PlatformStr, "platform", "", "the artifact platform in the format os[/arch][/variant][:os_version]")
	validateNodeCmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "use of HTTP protocol for all registry communications")
	validateNodeCmd.Flags().BoolVar(&outputJSON, "output-json", false, "print a JSON object")
	validateNodeCmd.Flags().StringVar(&username, "registry-username", "", "registry username")
	validateNodeCmd.Flags().BoolVar(&readPassword, "registry-password-stdin", false, "read registry password from stdin")
	validateNodeCmd.Flags().BoolVar(&readAccessToken, "registry-token-stdin", false, "read registry access token from stdin")

	if err := validateNodeCmd.MarkFlagRequired("image"); err != nil {
		panic(err)
	}
}
