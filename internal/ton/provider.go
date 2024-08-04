package ton

import "context"

type Provider interface {
	Balance(ctx context.Context, addr string) (int64, error)
	Send(ctx context.Context, from, to string, amount int64) (string, error)
	TxStatus(ctx context.Context, id string) (string, error)
}
