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

package system

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"strings"
)

type libcType string

const (
	glibcType   libcType = "glibc"
	muslType    libcType = "musl"
	unknownType libcType = "unknown"
)

const (
	libcVersionAttr = "version"
	libcNameAttr    = "name"
)

func getLibcAttributes() (map[string]string, error) {
	libcPath := getLibcPath()
	if libcPath == "" {
		return map[string]string{
			libcNameAttr:    string(unknownType),
			libcVersionAttr: "",
		}, nil
	}

	libcType, version, err := detectLibcImplementation(libcPath)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		libcNameAttr:    string(libcType),
		libcVersionAttr: version,
	}, nil
}

func getLibcPath() string {
	for _, path := range getCommonLibcPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// TODO: reading libc version from ELF is not reliable.
// For the better detection, we should execute ldd command with chroot inside a container.
// However, it requires changes to the host directories mount in the project.
// This solution is a workaround for the current situation.
func detectLibcImplementation(libcPath string) (libcType, string, error) {
	elfF, err := elf.Open(libcPath)
	if err != nil {
		return unknownType, "", fmt.Errorf("cannot open ELF binary: %w", err)
	}

	rodata := elfF.Section(".rodata")
	if rodata == nil {
		return unknownType, "", fmt.Errorf("missing.rodata section in %q", libcPath)
	}

	data, err := rodata.Data()
	if err != nil {
		return unknownType, "", fmt.Errorf("cannot read .rodata from %q", libcPath)
	}

	switch {
	case bytes.Contains(data, []byte("GNU C Library")):
		return glibcType, extractGlibcVersion(data), nil
	case bytes.Contains(data, []byte("musl")):
		// The version is not stored in ELF.
		// External solution has to be applied, like executing ldd directly.
		return muslType, "", nil
	default:
		return unknownType, "", nil
	}
}

func extractGlibcVersion(data []byte) string {
	lines := bytes.SplitSeq(data, []byte("\n"))
	for line := range lines {
		if bytes.Contains(line, []byte("release version")) {
			words := bytes.Fields(line)
			if len(words) > 0 {
				version, _ := strings.CutSuffix(string(words[len(words)-1]), ".")
				return version
			}
		}
	}
	return ""
}
