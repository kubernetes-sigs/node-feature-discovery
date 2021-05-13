package kubeconf

import (
	"path/filepath"
	"testing"
)

type testCaseData struct {
	path     string
	tmPolicy string
}

func TestGetKubeletConfigFromLocalFile(t *testing.T) {
	tCases := []testCaseData{
		{
			path:     filepath.Join("..", "..", "test", "data", "kubeletconf.yaml"),
			tmPolicy: "single-numa-node",
		},
	}

	for _, tCase := range tCases {
		cfg, err := GetKubeletConfigFromLocalFile(tCase.path)
		if err != nil {
			t.Errorf("failed to read config from %q: %v", tCase.path, err)
		}
		if cfg.TopologyManagerPolicy != tCase.tmPolicy {
			t.Errorf("TM policy mismatch, found %q expected %q", cfg.TopologyManagerPolicy, tCase.tmPolicy)
		}
	}
}
