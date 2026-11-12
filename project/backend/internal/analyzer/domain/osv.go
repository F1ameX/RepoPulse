package domain

import (
	"encoding/json"
	"time"
)

const OSVScope = "osv"

type OSVQuery struct {
	Ecosystem, Name, Version string
}

// OSVPage is a transient successful response. Only normalized facts derived
// from it go into the snapshot; upstream JSON and page tokens stay in memory.
type OSVPage struct {
	Body             json.RawMessage
	VulnerabilityIDs []string
	ObtainedAt       time.Time
}

type OSVResult struct {
	Query      OSVQuery
	Pages      []OSVPage
	ReasonCode string // empty only when all pages of this query were collected
}
