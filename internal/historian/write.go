package historian

import (
	"context"
	"github.com/natun-ai/natun/pkg/api"
)

func (h *historian) dispatchWrite(ctx context.Context, notification api.WriteNotification) error {
	//TODO write to s3
	return nil
}
