// Package di contains dependency injection tokens for the arbitrage context.
package di

import (
	"github.com/fd1az/arbitrage-bot/business/arbitrage/app"
	"github.com/fd1az/arbitrage-bot/internal/di"
)

// Public service tokens - exposed to other modules
var (
	Detector = di.NewToken[*app.Detector]("arbitrage.Detector")
)

// Private dependency tokens - internal to arbitrage module
var (
	ProfitCalculator = di.NewToken[*app.ProfitCalculator]("arbitrage:profitCalculator")
	Reporter         = di.NewToken[app.Reporter]("arbitrage:reporter")
)

// Helper functions for type-safe access
func GetDetector(c di.ServiceRegistry) *app.Detector {
	return di.GetToken(c, Detector)
}

func GetProfitCalculator(c di.ServiceRegistry) *app.ProfitCalculator {
	return di.GetToken(c, ProfitCalculator)
}

func GetReporter(c di.ServiceRegistry) app.Reporter {
	return di.GetToken(c, Reporter)
}
