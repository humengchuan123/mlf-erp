package auth

import "context"

type Actor struct {
	UserID       string
	UserName     string
	TenantID     string
	OrgID        string
	Roles        []string
	Permissions  []string
	IsSuperAdmin bool
}

type contextKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, contextKey{}, actor)
}

func ActorFrom(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(contextKey{}).(Actor)
	return actor, ok
}

func (a Actor) HasPermission(code string) bool {
	if a.IsSuperAdmin {
		return true
	}
	for _, permission := range a.Permissions {
		if permission == code {
			return true
		}
	}
	return false
}
