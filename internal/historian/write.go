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

package historian

import (
	"context"
	"github.com/natun-ai/natun/api"
	"sync/atomic"
)

func (h *historian) dispatchWrite(ctx context.Context, notification api.WriteNotification) error {
	atomic.AddUint32(&h.writes, 1)
	nv, err := api.NormalizeAny(notification.Value.Value)
	if err != nil {
		return err
	}
	notification.Value.Value = nv
	return h.HistoricalWriter.Commit(ctx, notification)
}

func (h *historian) finalizeWrite(ctx context.Context) {
	err := h.HistoricalWriter.FlushAll(ctx)
	if err != nil {
		h.Logger.Error(err, "failed to flush historical logs to storage")
	} else if h.writes > 0 {
		h.Logger.Info("successfully flushed historical logs to storage", "writes", h.writes)
		atomic.StoreUint32(&h.writes, 0)
	}
}
