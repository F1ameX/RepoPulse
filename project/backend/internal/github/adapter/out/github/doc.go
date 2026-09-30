// Package github is reserved for the outgoing GitHub platform adapter.
//
// The adapter will normalize GitHub DTOs, pagination, rate limits and collection
// completeness inside the GitHub API Service. HTTP handlers must not call it
// directly. This scaffold makes no outbound GitHub requests.
package github
