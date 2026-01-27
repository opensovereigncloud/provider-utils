// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pci

import (
	"github.com/go-logr/logr"
)

type reader struct {
	log logr.Logger
}

func NewReader(log logr.Logger, _ Vendor, _ Class) (*reader, error) {
	log.V(1).Info("NOT SUPPORTED OS")

	return &reader{
		log: log,
	}, nil

}

func (r *reader) Read() ([]Address, error) {
	r.log.V(1).Info("NOT SUPPORTED OS")
	return nil, nil
}
