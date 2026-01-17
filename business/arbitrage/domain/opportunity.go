// Package domain contains the core domain types for the arbitrage context.
package domain

import (
	"time"

	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/shopspring/decimal"
)

// ExecutionStep represents a step in the arbitrage execution plan.
type ExecutionStep struct {
	Number      int
	Description string
}

// RiskFactor represents a risk factor for an arbitrage opportunity.
type RiskFactor struct {
	Name        string
	Description string
	Severity    string // "low", "medium", "high"
}

// Opportunity represents a detected arbitrage opportunity.
type Opportunity struct {
	ID              string
	BlockNumber     uint64
	Timestamp       time.Time
	Pair            pricingDomain.Pair
	Direction       Direction
	TradeSize       decimal.Decimal
	CEXPrice        decimal.Decimal
	DEXPrice        decimal.Decimal
	Spread          pricingDomain.Spread
	GasCost         *GasCost
	Profit          *ProfitResult
	DEXQuote        *pricingDomain.Quote
	ExecutionSteps  []ExecutionStep
	RiskFactors     []RiskFactor
	RequiredCapital decimal.Decimal
}

// IsProfitable returns true if this opportunity has positive net profit.
func (o *Opportunity) IsProfitable() bool {
	return o.Profit != nil && o.Profit.IsProfitable
}
