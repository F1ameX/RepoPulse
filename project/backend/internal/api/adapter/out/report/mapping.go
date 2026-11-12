package report

import (
	"encoding/json"
	"time"

	models "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	common "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
	wire "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func fromReport(v *wire.Report) models.Report {
	r := models.Report{}
	if v == nil {
		return r
	}
	r.ID = v.ReportId
	r.AnalysisID = v.AnalysisId
	r.SnapshotID = v.SnapshotId
	r.Repository = fromRepositoryMetadata(v.Repository)
	r.AssessmentAt = v.AssessmentAt.AsTime().UTC()
	r.Window = fromWindow(v.Window)
	r.CompletedAt = v.CompletedAt.AsTime().UTC()
	r.Versions = fromVersions(v.Versions)
	r.Sources = []models.SourceState{}
	for _, item := range v.Sources {
		r.Sources = append(r.Sources, fromSourceState(item))
	}
	r.Dependencies = []models.Dependency{}
	for _, item := range v.Dependencies {
		r.Dependencies = append(r.Dependencies, fromDependency(item))
	}
	r.OverallScore = v.OverallScore
	if v.OverallReason != nil {
		item := fromReason(v.OverallReason)
		r.OverallReason = &item
	}
	r.Summary = v.Summary
	r.Coverage = fromCoverage(v.Coverage)
	r.Categories = []models.Category{}
	for _, item := range v.Categories {
		r.Categories = append(r.Categories, fromCategory(item))
	}
	r.Findings = []models.Finding{}
	for _, item := range v.Findings {
		r.Findings = append(r.Findings, fromFinding(item))
	}
	r.Recommendations = []models.Recommendation{}
	for _, item := range v.Recommendations {
		r.Recommendations = append(r.Recommendations, fromRecommendation(item))
	}
	r.Explanation = fromExplanation(v.Explanation)
	return r
}

func fromMetric(v *wire.Metric) models.Metric {
	r := models.Metric{}
	if v == nil {
		return r
	}
	r.Input = fromMetricInput(v.Input)
	r.NormalizedScore = v.NormalizedScore
	r.RuleVersion = v.RuleVersion
	r.Rule = v.Rule
	r.Threshold = v.Threshold
	r.Explanation = v.Explanation
	return r
}

func fromCoverage(v *wire.Coverage) models.Coverage {
	r := models.Coverage{}
	if v == nil {
		return r
	}
	r.Calculated = int(v.Calculated)
	r.Total = int(v.Total)
	r.Percentage = v.Percentage
	r.Partial = int(v.Partial)
	r.Unavailable = int(v.Unavailable)
	r.NotApplicable = int(v.NotApplicable)
	return r
}

func fromCategory(v *wire.Category) models.Category {
	r := models.Category{}
	if v == nil {
		return r
	}
	r.ID = v.Id
	r.Score = v.Score
	r.Status = v.Status
	if v.Reason != nil {
		item := fromReason(v.Reason)
		r.Reason = &item
	}
	r.Coverage = fromCoverage(v.Coverage)
	r.EffectiveWeight = v.EffectiveWeight
	r.Explanation = v.Explanation
	r.Metrics = []models.Metric{}
	for _, item := range v.Metrics {
		r.Metrics = append(r.Metrics, fromMetric(item))
	}
	return r
}

func fromFinding(v *wire.Finding) models.Finding {
	r := models.Finding{}
	if v == nil {
		return r
	}
	r.ID = v.Id
	r.RuleID = v.RuleId
	r.Category = v.Category
	r.Title = v.Title
	r.Description = v.Description
	r.Severity = v.Severity
	r.PossibleConsequence = v.PossibleConsequence
	r.EvidenceIDs = []string{}
	for _, item := range v.EvidenceIds {
		r.EvidenceIDs = append(r.EvidenceIDs, item)
	}
	r.Vulnerabilities = []models.Vulnerability{}
	for _, item := range v.Vulnerabilities {
		r.Vulnerabilities = append(r.Vulnerabilities, fromVulnerability(item))
	}
	return r
}

func fromRecommendation(v *wire.Recommendation) models.Recommendation {
	r := models.Recommendation{}
	if v == nil {
		return r
	}
	r.ID = v.Id
	r.FindingID = v.FindingId
	r.Title = v.Title
	r.Action = v.Action
	r.ExpectedBenefit = v.ExpectedBenefit
	r.Priority = v.Priority
	r.Complexity = v.Complexity
	r.VariantID = v.VariantId
	r.Explanation = v.Explanation
	return r
}

func fromExplanation(v *wire.Explanation) models.Explanation {
	r := models.Explanation{}
	if v == nil {
		return r
	}
	r.Mode = v.Mode
	r.FallbackReason = v.FallbackReason
	r.Model = v.Model
	r.CatalogVersion = v.CatalogVersion
	r.SchemaVersion = v.SchemaVersion
	r.PromptVersion = v.PromptVersion
	return r
}

func fromRepositoryMetadata(v *common.Repository) models.RepositoryMetadata {
	r := models.RepositoryMetadata{}
	if v == nil {
		return r
	}
	r.Platform = v.Platform
	r.GitHubID = v.GithubId
	r.CanonicalURL = v.CanonicalUrl
	r.Owner = v.Owner
	r.Name = v.Name
	r.DefaultBranch = v.DefaultBranch
	r.CommitSHA = v.CommitSha
	return r
}

func fromWindow(v *common.Window) models.Window {
	r := models.Window{}
	if v == nil {
		return r
	}
	r.From = v.From.AsTime().UTC()
	r.Until = v.Until.AsTime().UTC()
	return r
}

func fromVersions(v *common.Versions) models.Versions {
	r := models.Versions{}
	if v == nil {
		return r
	}
	r.Schema = v.Schema
	r.Methodology = v.Methodology
	r.ScoringConfig = v.ScoringConfig
	r.CollectorProfile = v.CollectorProfile
	r.SecretRules = v.SecretRules
	r.RecommendationCatalog = v.RecommendationCatalog
	r.ExplanationSchema = v.ExplanationSchema
	r.Prompt = v.Prompt
	return r
}

func fromReason(v *common.Reason) models.Reason {
	r := models.Reason{}
	if v == nil {
		return r
	}
	r.Code = v.Code
	r.Message = v.Message
	r.CannotConclude = v.CannotConclude
	return r
}

func fromSample(v *common.Sample) models.Sample {
	r := models.Sample{}
	if v == nil {
		return r
	}
	r.Collected = fromCount(v.Collected)
	r.Total = fromCount(v.Total)
	r.Numerator = fromCount(v.Numerator)
	r.Denominator = fromCount(v.Denominator)
	r.Excluded = fromCount(v.Excluded)
	r.ExcludedReason = v.ExcludedReason
	return r
}

func fromSourceState(v *common.SourceState) models.SourceState {
	r := models.SourceState{}
	if v == nil {
		return r
	}
	r.Source = v.Source
	r.Scope = v.Scope
	r.Status = v.Status
	if v.Reason != nil {
		item := fromReason(v.Reason)
		r.Reason = &item
	}
	r.Sample = fromSample(v.Sample)
	r.ObtainedAt = fromTime(v.ObtainedAt)
	r.Fingerprint = v.Fingerprint
	r.MetricIDs = v.MetricIds
	return r
}

func fromDependencyOrigin(v *common.DependencyOrigin) models.DependencyOrigin {
	r := models.DependencyOrigin{}
	if v == nil {
		return r
	}
	r.ManifestPath = v.ManifestPath
	r.VersionSource = v.VersionSource
	return r
}

func fromDependency(v *common.Dependency) models.Dependency {
	r := models.Dependency{}
	if v == nil {
		return r
	}
	r.Ecosystem = v.Ecosystem
	r.Package = v.Package
	r.Version = v.Version
	r.ManifestPath = v.ManifestPath
	r.VersionSource = v.VersionSource
	r.Origins = []models.DependencyOrigin{}
	for _, item := range v.Origins {
		r.Origins = append(r.Origins, fromDependencyOrigin(item))
	}
	return r
}

func fromVulnerability(v *common.Vulnerability) models.Vulnerability {
	r := models.Vulnerability{}
	if v == nil {
		return r
	}
	r.ID = v.VulnerabilityId
	r.Ecosystem = v.Ecosystem
	r.Package = v.Package
	r.Version = v.Version
	r.Severity = v.Severity
	r.ObtainedAt = v.ObtainedAt.AsTime().UTC()
	r.EvidenceURL = v.EvidenceUrl
	return r
}

func fromEvidence(v *common.Evidence) models.Evidence {
	r := models.Evidence{}
	if v == nil {
		return r
	}
	r.ID = v.Id
	r.Source = v.Source
	r.Kind = v.Kind
	r.Description = v.Description
	r.Path = v.Path
	r.Revision = v.Revision
	r.URL = v.Url
	r.RunID = v.RunId
	r.Attempt = fromCount(v.Attempt)
	if v.Dependency != nil {
		item := fromDependency(v.Dependency)
		r.Dependency = &item
	}
	r.VulnerabilityID = v.VulnerabilityId
	r.SecretMatchType = v.SecretMatchType
	return r
}

func fromMetricInput(v *common.MetricInput) models.MetricInput {
	r := models.MetricInput{}
	if v == nil {
		return r
	}
	r.ID = v.Id
	r.RawValue = fromScalar(v.RawValue)
	r.Unit = v.Unit
	r.Status = v.Status
	if v.Reason != nil {
		item := fromReason(v.Reason)
		r.Reason = &item
	}
	if v.Window != nil {
		item := fromWindow(v.Window)
		r.Window = &item
	}
	r.Sample = fromSample(v.Sample)
	r.Series = []int{}
	for _, item := range v.Series {
		r.Series = append(r.Series, int(item))
	}
	r.Evidence = []models.Evidence{}
	for _, item := range v.Evidence {
		r.Evidence = append(r.Evidence, fromEvidence(item))
	}
	r.EvidenceLimited = v.EvidenceLimited
	r.EvidenceTotal = fromCount(v.EvidenceTotal)
	r.ObservedAt = fromTime(v.ObservedAt)
	r.Exclusions = []models.ExcludedCount{}
	for _, item := range v.Exclusions {
		r.Exclusions = append(r.Exclusions, fromExcludedCount(item))
	}
	return r
}

func fromScalar(v *structpb.Value) models.Scalar {
	var result models.Scalar
	if v != nil {
		body, _ := json.Marshal(v.AsInterface())
		_ = result.UnmarshalJSON(body)
	}
	return result
}
func fromTime(v *timestamppb.Timestamp) *time.Time {
	if v == nil {
		return nil
	}
	result := v.AsTime().UTC()
	return &result
}
func fromCount(v *int32) *int {
	if v == nil {
		return nil
	}
	result := int(*v)
	return &result
}

func fromExcludedCount(v *common.ExcludedCount) models.ExcludedCount {
	if v == nil {
		return models.ExcludedCount{}
	}
	return models.ExcludedCount{Reason: v.Reason, Count: int(v.Count)}
}
