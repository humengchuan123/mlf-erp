package entitlement

import "context"

type Provider interface {
	Has(ctx context.Context, capability string) bool
}

type StaticProvider struct {
	capabilities []string
}

func NewStatic(capabilities ...string) *StaticProvider {
	return &StaticProvider{
		capabilities: append([]string(nil), capabilities...),
	}
}

func (p *StaticProvider) Has(_ context.Context, capability string) bool {
	for _, item := range p.capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func (p *StaticProvider) List() []string {
	return append([]string(nil), p.capabilities...)
}

var DefaultCapabilities = []string{
	"platform.org",
	"platform.rbac",
	"purchase",
	"sales",
	"inventory",
	"finance",
	"manufacturing",
	"quality",
	"project",
	"reporting",
}
