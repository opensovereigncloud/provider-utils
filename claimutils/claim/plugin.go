// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package claim

import (
	"errors"

	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	ErrInsufficientResources = errors.New("insufficient resources")
	ErrInvalidResourceClaim  = errors.New("invalid resource claim")
)

type Plugin interface {
	CanClaim(quantity resource.Quantity) bool
	Claim(quantity resource.Quantity) (ResourceClaim, error)
	Release(claim ResourceClaim) error
	Init() error
	Name() string
}

type ResourceClaim interface{}
