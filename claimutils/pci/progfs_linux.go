// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pci

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/prometheus/procfs/sysfs"
)

type reader struct {
	log logr.Logger
	fs  sysfs.FS

	vendorFilter Vendor
	classFilter  Class
}

func NewReader(log logr.Logger, vendorFilter Vendor, classFilter Class) (*reader, error) {
	fs, err := sysfs.NewDefaultFS()
	if err != nil {
		return nil, fmt.Errorf("failed to open sysfs: %w", err)
	}

	return &reader{
		log:          log,
		fs:           fs,
		vendorFilter: vendorFilter,
		classFilter:  classFilter,
	}, nil

}

func NewReaderWithMount(log logr.Logger, mountPoint string, vendorFilter Vendor, classFilter Class) (*reader, error) {
	fs, err := sysfs.NewFS(mountPoint)
	if err != nil {
		return nil, fmt.Errorf("failed to open sysfs: %w", err)
	}

	return &reader{
		log:          log,
		fs:           fs,
		vendorFilter: vendorFilter,
		classFilter:  classFilter,
	}, nil

}

func (r *reader) Read() ([]Address, error) {
	devices, err := r.fs.PciDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to read pci devices: %w", err)
	}

	var pciDevices []Address
	for _, device := range devices {
		switch {
		case device.Class != uint32(r.classFilter):
			r.log.V(3).Info(
				"Skipping device, class not matching",
				"device", device.Name(), "expected class",
				r.classFilter, "found class", device.Class,
			)
			continue
		case device.Vendor != uint32(r.vendorFilter):
			r.log.V(3).Info(
				"Skipping device, vendor not matching",
				"device", device.Name(), "expected vendor",
				r.vendorFilter, "found vendor", device.Vendor,
			)
			continue
		}

		r.log.V(1).Info("Found matching pci device", "device", device.Name())
		pciDevices = append(pciDevices, Address{
			Domain:   uint(device.Location.Segment),
			Bus:      uint(device.Location.Bus),
			Slot:     uint(device.Location.Device),
			Function: uint(device.Location.Function),
		})

	}

	return pciDevices, nil
}
