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
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

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
