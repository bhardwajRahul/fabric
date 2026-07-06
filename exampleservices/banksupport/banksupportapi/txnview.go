package banksupportapi

// TxnView is a customer-facing projection of a single transaction, returned by the Transactions tool.
type TxnView struct {
	Date        string `json:"date,omitzero" jsonschema_description:"Date is the transaction date in YYYY-MM-DD format"`
	AmountCents int    `json:"amountCents,omitzero" jsonschema_description:"AmountCents is the signed amount in cents; positive for credits, negative for debits"`
	Category    string `json:"category,omitzero" jsonschema_description:"Category is the spending category, e.g. groceries, dining, transport"`
	Description string `json:"description,omitzero" jsonschema_description:"Description is a short memo for the transaction"`
}
