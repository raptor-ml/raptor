package historian

import (
	"context"
	"github.com/natun-ai/natun/pkg/api"
)

func (h *historian) dispatchWrite(ctx context.Context, notification api.WriteNotification) error {
	return h.HistoricalWriter.Commit(ctx, notification)
}

func (h *historian) finalizeWrite(ctx context.Context) {
	err := h.HistoricalWriter.FlushAll(ctx)
	if err != nil {
		h.Logger.Error(err, "failed to flush historical logs to storage")
	}
}
