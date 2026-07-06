package banksupport

import (
	"hash/fnv"
	"math/rand"
	"slices"
	"time"

	"github.com/microbus-io/fabric/exampleservices/banksupport/banksupportapi"
)

// backfillWindow is how much transaction history is synthesized for a demo account.
const backfillWindow = 120 * 24 * time.Hour

// discretionaryJitter is the fractional range applied to variable expenses (groceries, dining, transport) so the
// ledger reads like a real statement rather than a metronome. Fixed obligations (salary, rent, utilities) are left
// unjittered so bob reliably trends into overdraft.
const discretionaryJitter = 0.2

// demoAccount is a seeded, read-only demo account held in memory. The map of accounts is built once in OnStartup
// and never mutated afterward, so it needs no lock.
type demoAccount struct {
	holder       string
	balanceCents int
	txns         []banksupportapi.TxnView // most-recent-first
	txnDates     []time.Time             // parallel to txns, for range filtering
}

// profile describes a demo customer and the spending pattern used to synthesize their transaction history.
// Amounts are in cents; expenses are magnitudes that are debited (subtracted).
type profile struct {
	holder     string
	startCents int
	salary     int // monthly credit
	rent       int // monthly debit
	groceries  int // weekly debit
	dining     int // weekly debit
	transport  int // weekly debit
	utilities  int // monthly debit
}

// demoCustomers are the hardcoded logins the demo accepts. Alice runs a healthy surplus; Bob spends beyond his
// income and trends into overdraft, which exercises the agent's risk assessment and card-block recommendation.
var demoCustomers = map[string]profile{
	"alice": {holder: "Alice Anderson", startCents: 50000, salary: 300000, rent: 120000, groceries: 8000, dining: 4000, transport: 1500, utilities: 12000},
	"bob":   {holder: "Bob Baker", startCents: 30000, salary: 180000, rent: 140000, groceries: 9500, dining: 7000, transport: 2500, utilities: 15000},
}

// populateDemoData builds the in-memory demo store: every customer's account and a backfilled transaction history,
// synthesized deterministically per username. It runs once from OnStartup, after which the map is read-only.
func (svc *Service) populateDemoData() {
	now := time.Now()
	svc.accounts = make(map[string]*demoAccount, len(demoCustomers))
	for username, cust := range demoCustomers {
		views, dates, delta := generateTransactions(cust, now.Add(-backfillWindow), now, seededRand(username))
		svc.accounts[username] = &demoAccount{
			holder:       cust.holder,
			balanceCents: cust.startCents + delta,
			txns:         views,
			txnDates:     dates,
		}
	}
}

// seededRand returns a random source seeded deterministically from the username, so each customer's synthesized
// history is varied but reproducible across runs.
func seededRand(username string) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(username))
	return rand.New(rand.NewSource(int64(h.Sum64())))
}

// generateTransactions synthesizes a customer's ledger for the (from, to] window following their spending profile,
// returning the transaction views most-recent-first, their parallel occurrence dates, and the net change in
// balance in cents. Fixed obligations are constant; discretionary expenses are jittered using rng so the history
// looks organic.
func generateTransactions(p profile, from time.Time, to time.Time, rng *rand.Rand) (views []banksupportapi.TxnView, dates []time.Time, deltaCents int) {
	add := func(amount int, category, description string, at time.Time) {
		views = append(views, banksupportapi.TxnView{
			Date:        at.Format("2006-01-02"),
			AmountCents: amount,
			Category:    category,
			Description: description,
		})
		dates = append(dates, at)
		deltaCents += amount
	}
	jitter := func(amount int) int {
		factor := 1 - discretionaryJitter + rng.Float64()*2*discretionaryJitter
		return int(float64(amount) * factor)
	}
	day := time.Date(from.Year(), from.Month(), from.Day(), 12, 0, 0, 0, from.Location()).AddDate(0, 0, 1)
	for !day.After(to) {
		switch day.Day() {
		case 1:
			add(p.salary, "salary", "Payroll deposit", day)
			add(-p.rent, "housing", "Monthly rent", day)
		case 15:
			add(-p.utilities, "utilities", "Utilities bill", day)
		}
		switch day.Weekday() {
		case time.Monday:
			add(-jitter(p.groceries), "groceries", "Groceries", day)
		case time.Wednesday:
			add(-jitter(p.transport), "transport", "Transit pass", day)
		case time.Friday:
			add(-jitter(p.dining), "dining", "Dining out", day)
		}
		day = day.AddDate(0, 0, 1)
	}
	// Reverse into most-recent-first order.
	slices.Reverse(views)
	slices.Reverse(dates)
	return views, dates, deltaCents
}
