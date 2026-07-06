package banksupportapi

// Actor is the subset of the access-token claims the microservice reads to identify the signed-in customer.
type Actor struct {
	Subject string   `json:"sub"`
	Name    string   `json:"name"`
	Roles   []string `json:"roles"`
}
