package grpc

import (
	"encoding/json"
	"time"

	models "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	common "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
	wire "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toReport(v models.Report) *wire.Report {
	r := &wire.Report{}
	r.ReportId = v.ID
	r.AnalysisId = v.AnalysisID
	r.SnapshotId = v.SnapshotID
	r.Repository = toRepositoryMetadata(v.Repository)
	r.AssessmentAt = timestamppb.New(v.AssessmentAt)
	r.Window = toWindow(v.Window)
	r.CompletedAt = timestamppb.New(v.CompletedAt)
	r.Versions = toVersions(v.Versions)
	r.Sources = []*common.SourceState{}
	for _, item := range v.Sources {
		r.Sources = append(r.Sources, toSourceState(item))
	}
	r.Dependencies = []*common.Dependency{}
	for _, item := range v.Dependencies {
		r.Dependencies = append(r.Dependencies, toDependency(item))
	}
	r.OverallScore = v.OverallScore
	if v.OverallReason != nil {
		r.OverallReason = toReason(*v.OverallReason)
	}
	r.Summary = v.Summary
	r.Coverage = toCoverage(v.Coverage)
	r.Categories = []*wire.Category{}
	for _, item := range v.Categories {
		r.Categories = append(r.Categories, toCategory(item))
	}
	r.Findings = []*wire.Finding{}
	for _, item := range v.Findings {
		r.Findings = append(r.Findings, toFinding(item))
	}
	r.Recommendations = []*wire.Recommendation{}
	for _, item := range v.Recommendations {
		r.Recommendations = append(r.Recommendations, toRecommendation(item))
	}
	r.Explanation = toExplanation(v.Explanation)
	return r
}

func toMetric(v models.Metric) *wire.Metric {
	r := &wire.Metric{}
	r.Input = toMetricInput(v.Input)
	r.NormalizedScore = v.NormalizedScore
	r.RuleVersion = v.RuleVersion
	r.Rule = v.Rule
	r.Threshold = v.Threshold
	r.Explanation = v.Explanation
	return r
}

func toCoverage(v models.Coverage) *wire.Coverage {
	r := &wire.Coverage{}
	r.Calculated = int32(v.Calculated)
	r.Total = int32(v.Total)
	r.Percentage = v.Percentage
	r.Partial = int32(v.Partial)
	r.Unavailable = int32(v.Unavailable)
	r.NotApplicable = int32(v.NotApplicable)
	return r
}

func toCategory(v models.Category) *wire.Category {
	r := &wire.Category{}
	r.Id = v.ID
	r.Score = v.Score
	r.Status = v.Status
	if v.Reason != nil {
		r.Reason = toReason(*v.Reason)
	}
	r.Coverage = toCoverage(v.Coverage)
	r.EffectiveWeight = v.EffectiveWeight
	r.Explanation = v.Explanation
	r.Metrics = []*wire.Metric{}
	for _, item := range v.Metrics {
		r.Metrics = append(r.Metrics, toMetric(item))
	}
	return r
}

func toFinding(v models.Finding) *wire.Finding {
	r := &wire.Finding{}
	r.Id = v.ID
	r.RuleId = v.RuleID
	r.Category = v.Category
	r.Title = v.Title
	r.Description = v.Description
	r.Severity = v.Severity
	r.PossibleConsequence = v.PossibleConsequence
	r.EvidenceIds = []string{}
	for _, item := range v.EvidenceIDs {
		r.EvidenceIds = append(r.EvidenceIds, item)
	}
	r.Vulnerabilities = []*common.Vulnerability{}
	for _, item := range v.Vulnerabilities {
		r.Vulnerabilities = append(r.Vulnerabilities, toVulnerability(item))
	}
	return r
}

func toRecommendation(v models.Recommendation) *wire.Recommendation {
	r := &wire.Recommendation{}
	r.Id = v.ID
	r.FindingId = v.FindingID
	r.Title = v.Title
	r.Action = v.Action
	r.ExpectedBenefit = v.ExpectedBenefit
	r.Priority = v.Priority
	r.Complexity = v.Complexity
	r.VariantId = v.VariantID
	r.Explanation = v.Explanation
	return r
}

func toExplanation(v models.Explanation) *wire.Explanation {
	r := &wire.Explanation{}
	r.Mode = v.Mode
	r.FallbackReason = v.FallbackReason
	r.Model = v.Model
	r.CatalogVersion = v.CatalogVersion
	r.SchemaVersion = v.SchemaVersion
	r.PromptVersion = v.PromptVersion
	return r
}

func toRepositoryMetadata(v models.RepositoryMetadata) *common.Repository {
	r := &common.Repository{}
	r.Platform = v.Platform
	r.GithubId = v.GitHubID
	r.CanonicalUrl = v.CanonicalURL
	r.Owner = v.Owner
	r.Name = v.Name
	r.DefaultBranch = v.DefaultBranch
	r.CommitSha = v.CommitSHA
	return r
}

func toWindow(v models.Window) *common.Window {
	r := &common.Window{}
	r.From = timestamppb.New(v.From)
	r.Until = timestamppb.New(v.Until)
	return r
}

func toVersions(v models.Versions) *common.Versions {
	r := &common.Versions{}
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

func toReason(v models.Reason) *common.Reason {
	r := &common.Reason{}
	r.Code = v.Code
	r.Message = v.Message
	r.CannotConclude = v.CannotConclude
	return r
}

func toSample(v models.Sample) *common.Sample {
	r := &common.Sample{}
	r.Collected = toCount(v.Collected)
	r.Total = toCount(v.Total)
	r.Numerator = toCount(v.Numerator)
	r.Denominator = toCount(v.Denominator)
	r.Excluded = toCount(v.Excluded)
	r.ExcludedReason = v.ExcludedReason
	return r
}

func toSourceState(v models.SourceState) *common.SourceState {
	r := &common.SourceState{}
	r.Source = v.Source
	r.Scope = v.Scope
	r.Status = v.Status
	if v.Reason != nil {
		r.Reason = toReason(*v.Reason)
	}
	r.Sample = toSample(v.Sample)
	r.ObtainedAt = toTime(v.ObtainedAt)
	r.Fingerprint = v.Fingerprint
	r.MetricIds = v.MetricIDs
	return r
}

func toDependencyOrigin(v models.DependencyOrigin) *common.DependencyOrigin {
	r := &common.DependencyOrigin{}
	r.ManifestPath = v.ManifestPath
	r.VersionSource = v.VersionSource
	return r
}

func toDependency(v models.Dependency) *common.Dependency {
	r := &common.Dependency{}
	r.Ecosystem = v.Ecosystem
	r.Package = v.Package
	r.Version = v.Version
	r.ManifestPath = v.ManifestPath
	r.VersionSource = v.VersionSource
	r.Origins = []*common.DependencyOrigin{}
	for _, item := range v.Origins {
		r.Origins = append(r.Origins, toDependencyOrigin(item))
	}
	return r
}

func toVulnerability(v models.Vulnerability) *common.Vulnerability {
	r := &common.Vulnerability{}
	r.VulnerabilityId = v.ID
	r.Ecosystem = v.Ecosystem
	r.Package = v.Package
	r.Version = v.Version
	r.Severity = v.Severity
	r.ObtainedAt = timestamppb.New(v.ObtainedAt)
	r.EvidenceUrl = v.EvidenceURL
	return r
}

func toEvidence(v models.Evidence) *common.Evidence {
	r := &common.Evidence{}
	r.Id = v.ID
	r.Source = v.Source
	r.Kind = v.Kind
	r.Description = v.Description
	r.Path = v.Path
	r.Revision = v.Revision
	r.Url = v.URL
	r.RunId = v.RunID
	r.Attempt = toCount(v.Attempt)
	if v.Dependency != nil {
		r.Dependency = toDependency(*v.Dependency)
	}
	r.VulnerabilityId = v.VulnerabilityID
	r.SecretMatchType = v.SecretMatchType
	return r
}

func toMetricInput(v models.MetricInput) *common.MetricInput {
	r := &common.MetricInput{}
	r.Id = v.ID
	r.RawValue = toScalar(v.RawValue)
	r.Unit = v.Unit
	r.Status = v.Status
	if v.Reason != nil {
		r.Reason = toReason(*v.Reason)
	}
	if v.Window != nil {
		r.Window = toWindow(*v.Window)
	}
	r.Sample = toSample(v.Sample)
	r.Series = []int32{}
	for _, item := range v.Series {
		r.Series = append(r.Series, int32(item))
	}
	r.Evidence = []*common.Evidence{}
	for _, item := range v.Evidence {
		r.Evidence = append(r.Evidence, toEvidence(item))
	}
	r.EvidenceLimited = v.EvidenceLimited
	r.EvidenceTotal = toCount(v.EvidenceTotal)
	r.ObservedAt = toTime(v.ObservedAt)
	r.Exclusions = []*common.ExcludedCount{}
	for _, item := range v.Exclusions {
		r.Exclusions = append(r.Exclusions, toExcludedCount(item))
	}
	return r
}

func toScalar(v models.Scalar) *structpb.Value {
	body, _ := v.MarshalJSON()
	var scalar any
	_ = json.Unmarshal(body, &scalar)
	result, _ := structpb.NewValue(scalar)
	return result
}
func toTime(v *time.Time) *timestamppb.Timestamp {
	if v == nil {
		return nil
	}
	return timestamppb.New(*v)
}
func toCount(v *int) *int32 {
	if v == nil {
		return nil
	}
	result := int32(*v)
	return &result
}

func toExcludedCount(v models.ExcludedCount) *common.ExcludedCount {
	return &common.ExcludedCount{Reason: v.Reason, Count: int32(v.Count)}
}
