package jpf

import (
	"context"
)

type ModelResponseCache interface {
	GetCachedResponse(ctx context.Context, salt string, inputs []Message) (bool, []Message, Message, error)
	SetCachedResponse(ctx context.Context, salt string, inputs []Message, aux []Message, out Message) error
}
