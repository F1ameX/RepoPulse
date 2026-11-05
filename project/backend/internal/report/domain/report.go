// Package domain uses the immutable published report model shared with Analyzer.
package domain

import analyzer "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"

type Report = analyzer.Report
type VerifiedContext = analyzer.VerifiedContext

var ErrNotFound = analyzer.ErrNotFound
