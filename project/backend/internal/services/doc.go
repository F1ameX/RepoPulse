// Package services is reserved for application use cases and service contracts.
//
// The API Service will call Auth, Analysis Orchestrator and Report Service via
// gRPC. Analysis orchestration, persistence and Kafka consumers belong to their
// respective service processes, not to HTTP handlers. No domain service is
// wired yet; reserved API endpoints return 501 until those services exist.
package services
