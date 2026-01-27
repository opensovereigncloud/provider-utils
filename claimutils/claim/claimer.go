// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package claim

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/ironcore-dev/ironcore/api/core/v1alpha1"
)

var (
	ErrMissingPlugins = errors.New("no plugin for resource")
	ErrReleaseClaim   = errors.New("failed to release claim")
	ErrAlreadyStarted = errors.New("claimer already started")
	ErrNotStarted     = errors.New("claimer not running")
)

type Claims map[v1alpha1.ResourceName]ResourceClaim

type Claimer interface {
	Claim(ctx context.Context, resources v1alpha1.ResourceList) (Claims, error)
	Release(ctx context.Context, claims Claims) error
	Start(ctx context.Context) error
	WaitUntilStarted(ctx context.Context) error
}

func NewResourceClaimer(log logr.Logger, plugins ...Plugin) (*claimer, error) {
	c := claimer{
		log:     log,
		plugins: map[string]Plugin{},

		toClaim:   make(chan claimReq, 1),
		toRelease: make(chan releaseReq, 1),

		started:  make(chan struct{}),
		shutdown: make(chan struct{}),
	}

	for _, plugin := range plugins {
		if _, existing := c.plugins[plugin.Name()]; existing {
			return nil, fmt.Errorf("plugin %s already exists", plugin.Name())
		}
		c.plugins[plugin.Name()] = plugin
	}

	for _, plugin := range c.plugins {
		if err := plugin.Init(); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

type claimer struct {
	log     logr.Logger
	plugins map[string]Plugin

	toClaim   chan claimReq
	toRelease chan releaseReq

	startOnce sync.Once
	started   chan struct{}
	shutdown  chan struct{}
}

type claimRes struct {
	claims Claims
	err    error
}

type claimReq struct {
	resources  v1alpha1.ResourceList
	resultChan chan claimRes
}

type releaseReq struct {
	claims     Claims
	resultChan chan error
}

func (c *claimer) start(ctx context.Context) {
	defer func() {
		for req := range c.toClaim {
			req.resultChan <- claimRes{err: ctx.Err()}
		}
		for req := range c.toRelease {
			req.resultChan <- ctx.Err()
		}

		close(c.toClaim)
		close(c.toRelease)
	}()

	close(c.started)

	for {
		select {
		case <-ctx.Done():
			close(c.shutdown)
			return
		case req := <-c.toClaim:
			res := claimRes{}
			res.claims, res.err = c.claim(req.resources)
			req.resultChan <- res

		case req := <-c.toRelease:
			if err := c.release(req.claims); err != nil {
				req.resultChan <- errors.Join(ErrReleaseClaim, err)
			} else {
				req.resultChan <- nil
			}
		}
	}
}

func (c *claimer) Start(ctx context.Context) error {
	var called bool
	c.startOnce.Do(func() {
		called = true
		go c.start(ctx)
	})

	if !called {
		return ErrAlreadyStarted
	}

	<-ctx.Done()

	return nil
}

func (c *claimer) ensureRunning() error {
	select {
	case <-c.started:
	default:
		return ErrNotStarted
	}

	select {
	case <-c.shutdown:
		return ErrNotStarted
	default:
	}

	return nil
}

func (c *claimer) claim(resources v1alpha1.ResourceList) (Claims, error) {
	var insufficientResourceErrors []error
	for resourceName := range resources {
		plugin := c.plugins[string(resourceName)]
		if !plugin.CanClaim(resources[resourceName]) {
			insufficientResourceErrors = append(
				insufficientResourceErrors,
				fmt.Errorf("insufficient resource for %s", resourceName),
			)
		}
	}
	if len(insufficientResourceErrors) > 0 {
		return nil, errors.Join(ErrInsufficientResources, errors.Join(insufficientResourceErrors...))
	}

	claims := map[v1alpha1.ResourceName]ResourceClaim{}
	for resourceName := range resources {
		plugin := c.plugins[string(resourceName)]

		claim, claimErr := plugin.Claim(resources[resourceName])
		if claimErr != nil {
			if err := c.release(claims); err != nil {
				c.log.Error(errors.Join(ErrReleaseClaim, err), "failed to release claim ")
			}
			return nil, claimErr
		}

		claims[resourceName] = claim
	}

	return claims, nil
}

func (c *claimer) checkPluginsForResources(resources v1alpha1.ResourceList) error {
	var missingPluginErrors []error
	for resourceName := range resources {
		if _, ok := c.plugins[string(resourceName)]; !ok {
			missingPluginErrors = append(missingPluginErrors, fmt.Errorf("plugin for resource %s not found", resourceName))
		}
	}
	if len(missingPluginErrors) > 0 {
		return errors.Join(missingPluginErrors...)
	}

	return nil
}

func (c *claimer) Claim(ctx context.Context, resources v1alpha1.ResourceList) (Claims, error) {
	if err := c.checkPluginsForResources(resources); err != nil {
		return nil, errors.Join(ErrMissingPlugins, err)
	}

	if err := c.ensureRunning(); err != nil {
		return nil, err
	}

	req := claimReq{
		resources:  resources,
		resultChan: make(chan claimRes, 1),
	}
	select {
	case c.toClaim <- req:
	case <-c.shutdown:
		return nil, ErrNotStarted
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-req.resultChan:
		return res.claims, res.err
	}
}

func (c *claimer) release(claims Claims) error {
	var releaseErrors []error
	for resourceName := range claims {
		plugin := c.plugins[string(resourceName)]

		if err := plugin.Release(claims[resourceName]); err != nil {
			releaseErrors = append(releaseErrors, err)
		}
	}
	if len(releaseErrors) > 0 {
		return errors.Join(releaseErrors...)
	}

	return nil
}

func (c *claimer) checkPluginsForClaims(claims Claims) error {
	var missingPluginErrors []error
	for resourceName := range claims {
		if _, ok := c.plugins[string(resourceName)]; !ok {
			missingPluginErrors = append(missingPluginErrors, fmt.Errorf("plugin for resource %s not found", resourceName))
		}
	}
	if len(missingPluginErrors) > 0 {
		return errors.Join(missingPluginErrors...)
	}

	return nil
}

func (c *claimer) Release(ctx context.Context, claims Claims) error {
	if err := c.checkPluginsForClaims(claims); err != nil {
		return errors.Join(ErrMissingPlugins, err)
	}

	if err := c.ensureRunning(); err != nil {
		return err
	}
	req := releaseReq{
		claims:     claims,
		resultChan: make(chan error, 1),
	}
	select {
	case c.toRelease <- req:
	case <-c.shutdown:
		return ErrNotStarted
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-req.resultChan:
		return res
	}
}

func (c *claimer) WaitUntilStarted(ctx context.Context) error {
	select {
	case <-c.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
