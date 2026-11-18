package axiom

import "sync"

// tokenValueHandoff passes token values from the axiom_token_stateless resource to
// the axiom_token ephemeral resource without going through the Terraform state.
//
// The Axiom API returns a token value only once, when the token is created or
// regenerated. The resource records that value here, and an ephemeral
// resource that references the token by ID picks it up later in the same
// provider process, which is the process that serves the rest of that apply.
// Values are never persisted: once the process exits they are gone.
//
// It is package-level because the framework constructs the resource and the
// ephemeral resource independently and only shares the API client with them.
type tokenValueHandoff struct {
	mu     sync.Mutex
	values map[string]string
}

var tokenHandoff = &tokenValueHandoff{values: map[string]string{}}

func (h *tokenValueHandoff) put(id, value string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.values[id] = value
}

func (h *tokenValueHandoff) get(id string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	value, ok := h.values[id]
	return value, ok
}
