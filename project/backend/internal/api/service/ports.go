// Package service is reserved for API application use cases and their ports.
//
// The API Service will call Auth, Analysis Orchestrator and Report Service via
// gRPC. Analysis orchestration, persistence and Kafka consumers belong to their
// respective service processes, not to HTTP handlers. No domain service is
// wired yet; reserved API endpoints return 501 until those services exist.
// Future ports belong in this package, with implementations under adapter/out.
package service
