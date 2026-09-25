package api

// UsageResponse decodes GET /billing/usage (STOMPY-2187): the caller's own
// combined-unit usage against their tier's cap. UnitsUsed is nil with
// UnavailableReason set when the meter cannot measure it — the door never
// invents a zero, and `stompy whoami` (STOMPY-2232) must not either.
type UsageResponse struct {
	Tier              string  `json:"tier"`
	UnitsUsed         *int    `json:"units_used"`
	Cap               *int    `json:"cap"`
	Window            *string `json:"window"`
	PeriodStart       *string `json:"period_start"`
	PeriodEnd         *string `json:"period_end"`
	UnavailableReason *string `json:"unavailable_reason"`
}

// GetUsage fetches the caller's own usage. Errors (network, 404 on an old
// server, etc.) are the caller's to decide whether to surface or swallow —
// whoami treats usage as best-effort and never fails over it.
func (c *Client) GetUsage() (*UsageResponse, error) {
	var resp UsageResponse
	if err := c.Get("/billing/usage", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
