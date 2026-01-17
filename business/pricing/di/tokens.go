// Package di contains dependency injection tokens for the pricing context.
package di

import (
	"github.com/fd1az/arbitrage-bot/business/pricing/app"
	"github.com/fd1az/arbitrage-bot/internal/di"
)

// Public service tokens - exposed to other modules
var (
	PricingService = di.NewToken[*app.PricingService]("pricing.PricingService")
)

// Private dependency tokens - internal to pricing module
var (
	CEXProvider = di.NewToken[app.CEXProvider]("pricing:cexProvider")
	DEXProvider = di.NewToken[app.DEXProvider]("pricing:dexProvider")
)

// Helper functions for type-safe access
func GetPricingService(c di.ServiceRegistry) *app.PricingService {
	return di.GetToken(c, PricingService)
}

func GetCEXProvider(c di.ServiceRegistry) app.CEXProvider {
	return di.GetToken(c, CEXProvider)
}

func GetDEXProvider(c di.ServiceRegistry) app.DEXProvider {
	return di.GetToken(c, DEXProvider)
}
