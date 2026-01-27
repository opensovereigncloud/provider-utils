// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package pci_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ironcore-dev/provider-utils/claimutils/pci"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func writeFakePCIDevice(t *testing.T, sysRoot, id string, vals map[string]string) {
	t.Helper()

	parent := "pci0000:00"
	devDir := filepath.Join(sysRoot, "devices", parent, id)
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", devDir, err)
	}

	required := []string{
		"class",
		"vendor",
		"device",
		"subsystem_vendor",
		"subsystem_device",
		"revision",
	}

	for _, f := range required {
		val, ok := vals[f]
		if !ok {
			t.Fatalf("missing required %s in vals", f)
		}
		path := filepath.Join(devDir, f)
		if err := os.WriteFile(path, []byte(val+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	busDevicesDir := filepath.Join(sysRoot, "bus", "pci", "devices")
	if err := os.MkdirAll(busDevicesDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", busDevicesDir, err)
	}

	linkPath := filepath.Join(busDevicesDir, id)
	target := filepath.Join("..", "..", "..", "devices", parent, id)

	_ = os.Remove(linkPath)

	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("symlink %s -> %s: %v", linkPath, target, err)
	}
}

func TestPCIReader_ReadFilters(t *testing.T) {
	tmpDir := t.TempDir()

	// matching device 1
	writeFakePCIDevice(t, tmpDir, "0000:17:00.0", map[string]string{
		"class":            "0x030200",
		"vendor":           "0x10de",
		"device":           "0x2901",
		"subsystem_vendor": "0x10de",
		"subsystem_device": "0x0001",
		"revision":         "0x1",
	})

	// matching device 2
	writeFakePCIDevice(t, tmpDir, "0000:97:00.0", map[string]string{
		"class":            "0x030200",
		"vendor":           "0x10de",
		"device":           "0x2902",
		"subsystem_vendor": "0x10de",
		"subsystem_device": "0x0002",
		"revision":         "0x1",
	})

	// non-matching device (wrong class/vendor)
	writeFakePCIDevice(t, tmpDir, "0000:00:00.0", map[string]string{
		"class":            "0x040000",
		"vendor":           "0x1000",
		"device":           "0xBEEF",
		"subsystem_vendor": "0x1000",
		"subsystem_device": "0x0003",
		"revision":         "0x1",
	})

	logger := log.Log.WithName("pci-test")

	reader, err := pci.NewReaderWithMount(logger, tmpDir, pci.VendorNvidia, pci.Class3DController)
	if err != nil {
		t.Fatalf("NewReaderWithMount: %v", err)
	}

	devices, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got, want := len(devices), 2; got != want {
		t.Fatalf("expected %d devices, got %d: %+v", want, got, devices)
	}
}
