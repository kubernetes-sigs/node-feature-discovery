/*
Copyright 2025 The Kubernetes Authors.

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

package template

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

type Helper struct {
	template *template.Template
}

// funcMap returns sprig's text-template FuncMap with the host-environment
// functions removed. env / expandenv / getHostByName are the only sprig
// builtins that read process state or outbound DNS — leaving the rest
// (date, random, string, list, etc.) available for legitimate use in
// label expansion.
func funcMap() template.FuncMap {
	m := sprig.TxtFuncMap()
	for _, name := range []string{"env", "expandenv", "getHostByName"} {
		delete(m, name)
	}
	return m
}

// MaxTemplateSize bounds the input to text/template.Parse to prevent stack-
// overflow aborts from recursive descent in text/template/parse. Per-action
// parser stack cost is ~1.7 KiB and minimum action width is ~5 bytes, so a
// 64 KiB cap bounds total stack growth at ~22 MiB — well below Go's 1 GiB
// goroutine stack ceiling.
const MaxTemplateSize = 64 * 1024

// ErrTemplateTooLarge is returned by NewHelper when the input template
// exceeds MaxTemplateSize. Exported so callers can distinguish this
// from generic parse errors via errors.Is.
var ErrTemplateTooLarge = errors.New("template exceeds maximum allowed size")

func NewHelper(name string) (*Helper, error) {
	if len(name) > MaxTemplateSize {
		return nil, fmt.Errorf("%w: %d bytes > %d limit", ErrTemplateTooLarge, len(name), MaxTemplateSize)
	}
	tmpl := template.New("").Funcs(funcMap()).Option("missingkey=error")
	tmpl, err := tmpl.Parse(name)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	return &Helper{template: tmpl}, nil
}

func (h *Helper) execute(data interface{}) (string, error) {
	var tmp bytes.Buffer
	if err := h.template.Execute(&tmp, data); err != nil {
		return "", err
	}
	return tmp.String(), nil
}

// ExpandMap is a helper for expanding a template in to a map of strings. Data
// after executing the template is expected to be key=value pairs separated by
// newlines.
func (h *Helper) ExpandMap(data interface{}) (map[string]string, error) {
	expanded, err := h.execute(data)
	if err != nil {
		return nil, err
	}

	// Split out individual key-value pairs
	out := make(map[string]string)
	for item := range strings.SplitSeq(expanded, "\n") {
		// Remove leading/trailing whitespace and skip empty lines
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			split := strings.SplitN(trimmed, "=", 2)
			if len(split) == 1 {
				return nil, fmt.Errorf("missing value in expanded template line %q, (format must be '<key>=<value>')", trimmed)
			}
			out[split[0]] = split[1]
		}
	}
	return out, nil
}
