/*
Copyright 2022 Natun.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package programregistry

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/jellydator/ttlcache/v3"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/natun-ai/natun/pkg/pyexp"
	"time"
)

var ErrAlreadyRegistered = fmt.Errorf("already registered")
var ErrNotFound = fmt.Errorf("not found")

// Registry is a registry of PyExp.
// The PyExp Registry is a cache of PyExp Programs
// This allows the runtime to store the program regardless to the feature definition, and to execute programs
// by passing their SHA1 hash.
type Registry interface {
	Register(program string) (sha1 string, err error)
	Get(sha1 string) (program pyexp.Runtime, err error)
}
type registry struct {
	cache  *ttlcache.Cache[string, pyexp.Runtime]
	engine api.Engine
}

// New creates a new PyExp Registry
func New(ctx context.Context, engine api.Engine) Registry {
	c := ttlcache.New[string, pyexp.Runtime](
		ttlcache.WithTTL[string, pyexp.Runtime](time.Hour * 24), //if program was not used in 24 hours, it will be removed
	)
	go c.Start()
	go func(ctx context.Context) {
		<-ctx.Done()
		c.Stop()
	}(ctx)

	return &registry{
		cache:  c,
		engine: engine,
	}
}

// Register a program
func (r *registry) Register(program string) (string, error) {
	if v := r.cache.Get(program); v != nil {
		return "", ErrAlreadyRegistered
	}

	rt, err := pyexp.New(program, r.engine)
	if err != nil {
		return "", fmt.Errorf("failed to create pyexp runtime: %w", err)
	}

	h := sha1.New()
	h.Write([]byte(program))
	sum := fmt.Sprintf("%x", h.Sum(nil))
	r.cache.Set(sum, rt, ttlcache.DefaultTTL)

	return sum, nil
}

// Get a program by its SHA1
func (r *registry) Get(id string) (pyexp.Runtime, error) {
	if v := r.cache.Get(id); v != nil {
		return v.Value(), nil
	}
	return nil, ErrNotFound
}
