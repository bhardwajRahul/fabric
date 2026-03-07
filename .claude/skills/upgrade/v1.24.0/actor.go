package act

import (
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/frame"
)

// Actor represents the authenticated entity associated with a request.
type Actor struct {
	// Standard claims
	Issuer           string    `json:"iss,omitzero"`
	IdentityProvider string    `json:"idp,omitzero"`
	Subject          string    `json:"sub,omitzero"`
	ExpirationTime   time.Time `json:"exp,omitzero"`

	// Identifiers
	TenantID int `json:"tid,omitzero"` // The user's tenant ID
	UserID   int `json:"uid,omitzero"` // The user's ID

	// Security claims
	Roles  []string `json:"roles,omitzero"`  // Roles associated with the user
	Groups []string `json:"groups,omitzero"` // Identifiers of groups the user belongs in
	Scope  string   `json:"scope,omitzero"`  // Space-separated list of scopes

	// User preferences
	GivenName  string `json:"given_name,omitzero"`
	FamilyName string `json:"family_name,omitzero"`
	TimeZone   string `json:"zoneinfo,omitzero"`
	Locale     string `json:"locale,omitzero"`
}

// Of extracts the actor from the HTTP request or context.
func Of(x any) (act Actor, err error) {
	_, err = frame.Of(x).ParseActor(&act)
	return act, errors.Trace(err)
}
