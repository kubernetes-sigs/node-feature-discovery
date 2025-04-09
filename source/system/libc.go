package system

import (
	"bytes"
	"fmt"
	"os"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

type libcType string

const (
	glibcType   libcType = "glibc"
	unknownType libcType = "unknown"
)

const (
	libcVersionAttr = "version"
	libcNameAttr    = "name"
)

func getLibcAttributes() (map[string]string, error) {
	attrs := map[string]string{
		libcNameAttr:    string(unknownType),
		libcVersionAttr: "",
	}

	libcPath := getLibcPath()
	if libcPath == "" {
		return attrs, nil
	}

	libcType, version, err := detectLibcImplementation(libcPath)
	if err != nil {
		return nil, err
	}

	switch libcType {
	case glibcType:
		attrs[libcNameAttr] = string(glibcType)
		attrs[libcVersionAttr] = version
		return attrs, nil
	default:
		return attrs, nil
	}
}

func getLibcPath() string {

	// check the most common libc paths
	paths := []string{
		hostpath.UsrDir.Path("lib64/libc.so.6"),
		hostpath.LibDir.Path("x86_64-linux-gnu/libc.so.6"),
		hostpath.LibDir.Path("libc.so.6"),
		// TODO: add musl detection
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func detectLibcImplementation(libcPath string) (libcType, string, error) {
	data, err := os.ReadFile(libcPath)
	if err != nil {
		return unknownType, "", fmt.Errorf("failed to read libc binary: %w", err)
	}

	if bytes.Contains(data, []byte("GNU C Library")) {
		return glibcType, extractGlibcVersion(data, ""), nil
	}
	return unknownType, "", nil

}

func extractGlibcVersion(data []byte, anchor string) string {
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		if bytes.Contains(line, []byte("release version")) {
			words := bytes.Fields(line)
			if len(words) > 0 {
				return string(words[len(words)-1])
			}
		}
	}
	return ""
}
