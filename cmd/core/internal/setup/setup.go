/*
Copyright (c) 2022 RaptorML authors.

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

package setup

import (
	"fmt"
	"net/http"
	"os"

	"golang.org/x/sync/errgroup"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var healthChecks []healthz.Checker

// HealthCheck is an aggregated health check for the entire system.
func HealthCheck(r *http.Request) error {
	g, ctx := errgroup.WithContext(r.Context())
	r = r.WithContext(ctx)
	for _, check := range healthChecks {
		check := check // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return check(r)
		})
	}
	return g.Wait()
}

var setupLog = ctrl.Log.WithName("setup")

func OrFail(err error, message string, keyAndValues ...any) {
	if err != nil {
		if setupLog.GetSink() == nil {
			_, _ = fmt.Fprint(os.Stderr, append([]any{"error", err, "message", message}, keyAndValues...)...)
		} else {
			setupLog.Error(err, message, keyAndValues...)
		}
		os.Exit(1)
	}
}
