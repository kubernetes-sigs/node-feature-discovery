package net

import (
	"fmt"
	"io/ioutil"
	"net"
	"testing"

	"os"
	"path/filepath"
	"reflect"
)

func TestSource_GetSpeedFeatures(t *testing.T) {
	interfaceSpeedSysFilePattern = fmt.Sprintf("%snfd/%%s/speed", os.TempDir())

	ut_metas := []struct {
		Interfaces []struct {
			net.Interface
			Speed string
		}
		Wanted []string
	}{
		{
			Interfaces: []struct {
				net.Interface
				Speed string
			}{
				{
					Interface: net.Interface{
						Name:  "lo",
						Flags: net.FlagUp | net.FlagLoopback,
					},
					Speed: "10000\n",
				},
				// interface is down
				{
					Interface: net.Interface{
						Name:  "eth0",
						Flags: 0,
					},
					Speed: "1000\n",
				},
			},
			Wanted: []string{},
		},
		{
			Interfaces: []struct {
				net.Interface
				Speed string
			}{
				{
					Interface: net.Interface{
						Name:  "lo",
						Flags: net.FlagUp | net.FlagLoopback,
					},
					Speed: "10000\n",
				},
				// interface is up and interface name is eth*
				{
					Interface: net.Interface{
						Name:  "eth0",
						Flags: net.FlagUp,
					},
					Speed: "1000\n",
				},
			},
			Wanted: []string{"speed_1000"},
		},
		{
			Interfaces: []struct {
				net.Interface
				Speed string
			}{
				{
					Interface: net.Interface{
						Name:  "lo",
						Flags: net.FlagUp | net.FlagLoopback,
					},
					Speed: "10000\n",
				},
				// interface is up and interface name is em*
				{
					Interface: net.Interface{
						Name:  "em0",
						Flags: net.FlagUp,
					},
					Speed: "1000\n",
				},
			},
			Wanted: []string{"speed_1000"},
		},
		{
			Interfaces: []struct {
				net.Interface
				Speed string
			}{
				{
					Interface: net.Interface{
						Name:  "lo",
						Flags: net.FlagUp | net.FlagLoopback,
					},
					Speed: "10000\n",
				},
				// interface is up and interface name is em*
				{
					Interface: net.Interface{
						Name:  "em0",
						Flags: net.FlagUp,
					},
					Speed: "1000\n",
				},
				// interface is up and interface name is em*
				{
					Interface: net.Interface{
						Name:  "em1",
						Flags: net.FlagUp,
					},
					Speed: "10000\n",
				},
			},
			Wanted: []string{"speed_1000", "speed_10000"},
		},
	}

	for _, ut_meta := range ut_metas {
		for _, iFace := range ut_meta.Interfaces {
			speedSysFile := fmt.Sprintf(interfaceSpeedSysFilePattern, iFace.Name)
			if _, err := os.Stat(speedSysFile); err != nil && os.IsExist(err) {
				err := os.Remove(speedSysFile)
				if err != nil {
					t.Errorf("delete test interface speed sys file path error. %v", err)
				}
			}
			err := os.MkdirAll(filepath.Dir(speedSysFile), os.ModeDir|os.ModePerm)
			if err != nil {
				t.Errorf("creat test interface speed sys file path error. %v", err)
			}

			err = ioutil.WriteFile(speedSysFile, []byte(iFace.Speed), os.ModePerm)
			if err != nil {
				t.Errorf("write test interface speed sys file error. %v", err)
			}
		}

		iFaces := []net.Interface{}
		for _, iFace := range ut_meta.Interfaces {
			iFaces = append(iFaces, iFace.Interface)
		}

		s := Source{}
		speedFeatures := s.getSpeedFeatures(iFaces)
		if !reflect.DeepEqual(speedFeatures, ut_meta.Wanted) {
			t.Logf("query net speed feature error. wanted: %v, got: %v", ut_meta.Wanted, speedFeatures)
		}
	}
}
