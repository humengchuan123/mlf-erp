package idempotency

import "time"

type Config struct {
	TTL              time.Duration
	Required         bool
	MaxBodyBytes     int64
	MaxResponseBytes int64
	ProtectedPaths   map[string]struct{}
}

var DefaultProtectedPaths = map[string]struct{}{
	"/api/v1/finance/credit-notes/:id/allocations": {},
	"/api/v1/finance/credit-notes/:id/refunds":     {},
	"/api/v1/finance/payments":                     {},
	"/api/v1/finance/payments/:id/allocate":        {},
	"/api/v1/finance/payments/:id/unallocate":      {},
	"/api/v1/sales/orders/:id/ship":                {},
	"/api/v1/purchase/orders/:id/receive":          {},
	"/api/v1/sales/shipments/:id/returns":          {},
	"/api/v1/purchase/receipts/:id/returns":        {},
}

func (c Config) withDefaults() Config {
	if c.TTL <= 0 {
		c.TTL = 24 * time.Hour
	}
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = 1 << 20
	}
	if c.MaxResponseBytes <= 0 {
		c.MaxResponseBytes = 4 << 20
	}
	if c.ProtectedPaths == nil {
		c.ProtectedPaths = DefaultProtectedPaths
	}
	return c
}
