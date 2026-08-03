// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/ironcore-dev/provider-utils/apiutils/api"
	"github.com/ironcore-dev/provider-utils/storeutils/store"
	utilssync "github.com/ironcore-dev/provider-utils/storeutils/sync"
	"github.com/ironcore-dev/provider-utils/storeutils/utils"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Options[E api.Object] struct {
	Dir             string
	NewFunc         func() E
	CreateStrategy  CreateStrategy[E]
	WatchBufferSize int
	FieldIndexers   map[string]store.IndexerFunc[E]
}

func (o *Options[E]) Defaults() {
	if o.WatchBufferSize <= 0 {
		o.WatchBufferSize = 20
	}
}

func NewStore[E api.Object](opts Options[E]) (*Store[E], error) {
	opts.Defaults()

	if opts.NewFunc == nil {
		return nil, fmt.Errorf("must specify opts.NewFunc")
	}

	if err := os.MkdirAll(opts.Dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating store directory: %w", err)
	}

	indexers := make(map[string]store.IndexerFunc[E], len(opts.FieldIndexers))
	for k, v := range opts.FieldIndexers {
		indexers[k] = v
	}
	return &Store[E]{
		dir: opts.Dir,

		idMu: utilssync.NewMutexMap[string](),

		newFunc:        opts.NewFunc,
		createStrategy: opts.CreateStrategy,

		watches:         sets.New[*watch[E]](),
		watchBufferSize: opts.WatchBufferSize,

		indexers: indexers,
	}, nil
}

type Store[E api.Object] struct {
	dir string

	idMu *utilssync.MutexMap[string]

	newFunc        func() E
	createStrategy CreateStrategy[E]

	watchBufferSize int
	watchesMu       sync.RWMutex
	watches         sets.Set[*watch[E]]

	indexers map[string]store.IndexerFunc[E]
}

type CreateStrategy[E api.Object] interface {
	PrepareForCreate(obj E)
}

func (s *Store[E]) Create(_ context.Context, obj E) (E, error) {
	s.idMu.Lock(obj.GetID())
	defer s.idMu.Unlock(obj.GetID())

	_, err := s.get(obj.GetID())
	switch {
	case err == nil:
		return utils.Zero[E](), fmt.Errorf("object with id %q %w", obj.GetID(), store.ErrAlreadyExists)
	case errors.Is(err, store.ErrNotFound):
	default:
		return utils.Zero[E](), fmt.Errorf("failed to get object with id %q %w", obj.GetID(), err)
	}

	if s.createStrategy != nil {
		s.createStrategy.PrepareForCreate(obj)
	}

	obj.SetCreatedAt(time.Now())
	obj.IncrementResourceVersion()

	obj, err = s.set(obj)
	if err != nil {
		return utils.Zero[E](), err
	}

	s.enqueue(store.WatchEvent[E]{
		Type:   store.WatchEventTypeCreated,
		Object: obj,
	})

	return obj, nil
}

func (s *Store[E]) Get(_ context.Context, id string) (E, error) {
	s.idMu.Lock(id)
	defer s.idMu.Unlock(id)

	object, err := s.get(id)
	if err != nil {
		return utils.Zero[E](), fmt.Errorf("failed to read object: %w", err)
	}

	return object, nil
}

func (s *Store[E]) Update(_ context.Context, obj E) (E, error) {
	s.idMu.Lock(obj.GetID())
	defer s.idMu.Unlock(obj.GetID())

	oldObj, err := s.get(obj.GetID())
	if err != nil {
		return utils.Zero[E](), err
	}

	if obj.GetDeletedAt() != nil && len(obj.GetFinalizers()) == 0 {
		if err := s.delete(obj); err != nil {
			return utils.Zero[E](), fmt.Errorf("failed to delete object metadata: %w", err)
		}
		return obj, nil
	}

	if oldObj.GetResourceVersion() != obj.GetResourceVersion() {
		return utils.Zero[E](), fmt.Errorf("failed to update object: %w", store.ErrResourceVersionNotLatest)
	}

	if reflect.DeepEqual(oldObj, obj) {
		return obj, nil
	}

	obj.IncrementResourceVersion()

	obj, err = s.set(obj)
	if err != nil {
		return utils.Zero[E](), err
	}

	s.enqueue(store.WatchEvent[E]{
		Type:   store.WatchEventTypeUpdated,
		Object: obj,
	})

	return obj, nil
}

func (s *Store[E]) Delete(_ context.Context, id string) error {
	s.idMu.Lock(id)
	defer s.idMu.Unlock(id)

	obj, err := s.get(id)
	if err != nil {
		return err
	}

	if len(obj.GetFinalizers()) == 0 {
		return s.delete(obj)
	}

	if obj.GetDeletedAt() != nil {
		return nil
	}

	now := time.Now()
	obj.SetDeletedAt(&now)
	obj.IncrementResourceVersion()

	if _, err := s.set(obj); err != nil {
		return fmt.Errorf("failed to set object metadata: %w", err)
	}

	s.enqueue(store.WatchEvent[E]{
		Type:   store.WatchEventTypeDeleted,
		Object: obj,
	})

	return nil
}

func (s *Store[E]) validateFieldSelector(opts store.ListOptions) error {
	if opts.FieldSelector == nil {
		return nil
	}
	for _, req := range opts.FieldSelector.Requirements() {
		if _, ok := s.indexers[req.Field]; !ok {
			return fmt.Errorf("field selector references unindexed field %q", req.Field)
		}
	}
	return nil
}

func (s *Store[E]) matchesOptions(obj E, opts store.ListOptions) bool {
	if opts.LabelSelector != nil && !opts.LabelSelector.Matches(labels.Set(obj.GetLabels())) {
		return false
	}
	if opts.FieldSelector != nil {
		merged := fields.Set{}
		for _, req := range opts.FieldSelector.Requirements() {
			if fn, ok := s.indexers[req.Field]; ok {
				merged[req.Field] = fn(obj)
			}
		}
		if !opts.FieldSelector.Matches(merged) {
			return false
		}
	}
	return true
}

func (s *Store[E]) List(ctx context.Context, opts ...store.ListOption) ([]E, error) {
	listOpts := &store.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(listOpts)
	}

	if err := s.validateFieldSelector(*listOpts); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	//nolint:prealloc
	var objs []E
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		object, err := s.Get(ctx, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read object: %w", err)
		}

		if !s.matchesOptions(object, *listOpts) {
			continue
		}

		objs = append(objs, object)
	}

	return objs, nil
}

func (s *Store[E]) Watch(_ context.Context, opts ...store.ListOption) (store.Watch[E], error) {
	listOpts := &store.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(listOpts)
	}

	if err := s.validateFieldSelector(*listOpts); err != nil {
		return nil, err
	}

	s.watchesMu.Lock()
	defer s.watchesMu.Unlock()

	w := &watch[E]{
		store:   s,
		events:  make(chan store.WatchEvent[E], s.watchBufferSize),
		opts:    *listOpts,
		members: sets.New[string](),
	}

	s.watches.Insert(w)

	return w, nil
}

func (s *Store[E]) get(id string) (E, error) {
	file, err := os.ReadFile(filepath.Join(s.dir, id))
	if err != nil {
		if !os.IsNotExist(err) {
			return utils.Zero[E](), fmt.Errorf("failed to read file: %w", err)
		}

		return utils.Zero[E](), fmt.Errorf("object with id %q %w", id, store.ErrNotFound)
	}

	obj := s.newFunc()
	if err := json.Unmarshal(file, &obj); err != nil {
		return utils.Zero[E](), fmt.Errorf("failed to unmarshal object from file %s: %w", id, err)
	}

	return obj, err
}

func (s *Store[E]) set(obj E) (E, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return utils.Zero[E](), fmt.Errorf("failed to marshal obj: %w", err)
	}

	if err := os.WriteFile(filepath.Join(s.dir, obj.GetID()), data, 0666); err != nil {
		return utils.Zero[E](), nil
	}

	return obj, nil
}

func (s *Store[E]) delete(obj E) error {
	if err := os.Remove(filepath.Join(s.dir, obj.GetID())); err != nil {
		return fmt.Errorf("failed to delete object from store: %w", err)
	}

	s.enqueue(store.WatchEvent[E]{
		Type:   store.WatchEventTypeDeleted,
		Object: obj,
	})

	return nil
}

func (s *Store[E]) watchHandlers() []*watch[E] {
	s.watchesMu.RLock()
	defer s.watchesMu.RUnlock()

	return s.watches.UnsortedList()
}

func (s *Store[E]) send(w *watch[E], evt store.WatchEvent[E]) {
	select {
	case w.events <- evt:
	default:
	}
}

func (s *Store[E]) enqueue(evt store.WatchEvent[E]) {
	id := evt.Object.GetID()
	for _, handler := range s.watchHandlers() {
		if handler.matches(evt.Object) {
			handler.members.Insert(id)
			s.send(handler, evt)
		} else if handler.members.Has(id) {
			// Object transitioned out of this watch's scope. Send deleted event.
			handler.members.Delete(id)
			s.send(handler, store.WatchEvent[E]{Type: store.WatchEventTypeDeleted, Object: evt.Object})
		}
	}
}
