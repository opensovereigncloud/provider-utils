// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type ListOptions struct {
	LabelSelector labels.Selector
	FieldSelector fields.Selector
}

type ListOption interface {
	ApplyToList(*ListOptions)
}

type MatchingLabels labels.Set

func (m MatchingLabels) ApplyToList(o *ListOptions) {
	sel := labels.SelectorFromSet(labels.Set(m))
	o.LabelSelector = andSelectors(o.LabelSelector, sel)
}

type HasLabels []string

func (h HasLabels) ApplyToList(o *ListOptions) {
	sel := labels.NewSelector()
	for _, key := range h {
		req, err := labels.NewRequirement(key, selection.Exists, nil)
		// Invalid labels are ignored, leading to an empty selector.
		// This is in line with the behavior of controller-runtime:
		// https://github.com/kubernetes-sigs/controller-runtime/blob/main/pkg/client/options.go#L637-L644
		if err != nil {
			continue
		}
		sel = sel.Add(*req)
	}
	o.LabelSelector = andSelectors(o.LabelSelector, sel)
}

func andSelectors(existing, additional labels.Selector) labels.Selector {
	if existing == nil {
		return additional
	}
	reqs, _ := additional.Requirements()
	return existing.Add(reqs...)
}

type MatchingFields fields.Set

func (m MatchingFields) ApplyToList(o *ListOptions) {
	sel := fields.Set(m).AsSelector()
	if o.FieldSelector == nil {
		o.FieldSelector = sel
	} else {
		o.FieldSelector = fields.AndSelectors(o.FieldSelector, sel)
	}
}
