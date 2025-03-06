// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ironcore-dev/controller-utils/metautils"
)

func Zero[E any]() E {
	var zero E
	return zero
}

func DeleteSliceElement[E comparable](s []E, elem E) []E {
	idx := slices.Index(s, elem)
	if idx < 0 {
		return s
	}

	return slices.Delete(s, idx, idx+1)
}

func SetLabelsAnnotation(o Object, key string, labels map[string]string) error {
	data, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("error marshalling labels: %w", err)
	}
	metautils.SetAnnotation(o, key, string(data))
	return nil
}

func GetLabelsAnnotation(o Metadata, key string) (map[string]string, error) {
	data, ok := o.GetAnnotations()[key]
	if !ok {
		return nil, fmt.Errorf("object has no labels at %s", key)
	}

	var labels map[string]string
	if err := json.Unmarshal([]byte(data), &labels); err != nil {
		return nil, err
	}

	return labels, nil
}

func SetAnnotationsAnnotation(o Object, key string, annotations map[string]string) error {
	data, err := json.Marshal(annotations)
	if err != nil {
		return fmt.Errorf("error marshalling annotations: %w", err)
	}
	metautils.SetAnnotation(o, key, string(data))

	return nil
}

func GetAnnotationsAnnotation(o Metadata, key string) (map[string]string, error) {
	data, ok := o.GetAnnotations()[key]
	if !ok {
		return nil, fmt.Errorf("object has no annotations at %s", key)
	}

	var annotations map[string]string
	if err := json.Unmarshal([]byte(data), &annotations); err != nil {
		return nil, err
	}

	return annotations, nil
}
