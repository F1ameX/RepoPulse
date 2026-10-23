// Package proto holds Go code generated from the service contracts in /contracts.
//
// Regenerate from project/backend (protoc must be installed):
//
//	go install tool
//	go generate ./pkg/proto
package proto

//go:generate protoc -I ../../../../contracts --go_out=. --go_opt=module=github.com/F1ameX/RepoPulse/project/backend/pkg/proto --go-grpc_out=. --go-grpc_opt=module=github.com/F1ameX/RepoPulse/project/backend/pkg/proto common/v1/common.proto auth/v1/auth.proto
