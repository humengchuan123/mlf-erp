package tenant

import "context"

type Info struct {
	ID    string
	OrgID string
}

type contextKey struct{}

func With(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, contextKey{}, info)
}

func From(ctx context.Context) (Info, bool) {
	info, ok := ctx.Value(contextKey{}).(Info)
	return info, ok
}

func ID(ctx context.Context) string {
	info, _ := From(ctx)
	return info.ID
}
