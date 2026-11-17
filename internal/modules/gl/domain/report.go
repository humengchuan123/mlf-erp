package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type TrialBalanceRow struct {
	AccountID    string
	Code         string
	Name         string
	Type         string
	Direction    string
	Opening      decimal.Decimal
	PeriodDebit  decimal.Decimal
	PeriodCredit decimal.Decimal
	Closing      decimal.Decimal
}

type TrialBalance struct {
	StartDate   time.Time
	EndDate     time.Time
	Rows        []*TrialBalanceRow
	TotalDebit  decimal.Decimal
	TotalCredit decimal.Decimal
}

type GeneralLedgerEntry struct {
	VoucherID   string
	VoucherCode string
	VoucherDate time.Time
	Summary     string
	Debit       decimal.Decimal
	Credit      decimal.Decimal
	Balance     decimal.Decimal
}

type GeneralLedger struct {
	Account   *Account
	StartDate time.Time
	EndDate   time.Time
	Opening   decimal.Decimal
	Entries   []*GeneralLedgerEntry
	Closing   decimal.Decimal
}

type ReportLine struct {
	Code   string
	Name   string
	Amount decimal.Decimal
}

type BalanceSheet struct {
	AsOf                      time.Time
	Assets                    []*ReportLine
	Liabilities               []*ReportLine
	Equity                    []*ReportLine
	TotalAssets               decimal.Decimal
	TotalLiabilities          decimal.Decimal
	TotalEquity               decimal.Decimal
	CurrentProfit             decimal.Decimal
	TotalLiabilitiesAndEquity decimal.Decimal
}

type IncomeStatement struct {
	StartDate     time.Time
	EndDate       time.Time
	Revenue       []*ReportLine
	Expenses      []*ReportLine
	TotalRevenue  decimal.Decimal
	TotalExpenses decimal.Decimal
	NetProfit     decimal.Decimal
}

type AuxiliaryBalanceRow struct {
	AccountID     string
	AccountCode   string
	AccountName   string
	DimensionID   string
	DimensionCode string
	DimensionName string
	Opening       decimal.Decimal
	PeriodDebit   decimal.Decimal
	PeriodCredit  decimal.Decimal
	Closing       decimal.Decimal
}

type AuxiliaryBalance struct {
	Dimension   string
	StartDate   time.Time
	EndDate     time.Time
	Rows        []*AuxiliaryBalanceRow
	TotalDebit  decimal.Decimal
	TotalCredit decimal.Decimal
}
