package scoring

import (
	"encoding/json"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const testExplanation = "Файловый признак тестов. Тесты не запускались, покрытие и успешность не измерялись."

// TreeInputs uses only paths and modes from the pinned Git tree. It retains
// observed evidence from a partial tree without assigning a raw value or score.
func (e *Engine) TreeInputs(git domain.GitData, tree domain.SourceState) []domain.MetricInput {
	var readme, support, tests []string
	var hasSource bool
	supportKinds := make(map[string]bool, len(e.config.SupportingDocuments))
	if tree.Status == "complete" || tree.Status == "partial" {
		for _, file := range git.Files {
			if file.Mode != "100644" && file.Mode != "100755" {
				continue
			}
			dir, name := path.Split(file.Path)
			if dir == "" {
				switch strings.ToLower(name) {
				case "readme", "readme.md", "readme.txt", "readme.rst":
					readme = append(readme, file.Path)
				}
			}
			if (dir == "" || dir == ".github/") && slices.Contains(e.config.SupportingDocuments, name) {
				supportKinds[name] = true
				support = append(support, file.Path)
			}
			isSource := sourceFile(name)
			hasSource = hasSource || isSource
			if testFile(dir, name, isSource) {
				tests = append(tests, file.Path)
			}
		}
	}
	readmeInput := treeInput("DOC_README", "bool", tree, git.Repository.CommitSHA, readme, len(readme))
	supportInput := treeInput("DOC_SUPPORT_SHARE", "ratio", tree, git.Repository.CommitSHA, support, len(support))
	if tree.Status == "complete" && !hasSource {
		tests = nil
	}
	testInput := treeInput("TEST_PRESENCE", "bool", tree, git.Repository.CommitSHA, tests, 10)
	if tree.Status == "complete" {
		_ = readmeInput.RawValue.UnmarshalJSON([]byte(strconv.FormatBool(len(readme) > 0)))
		count, total := len(supportKinds), len(e.config.SupportingDocuments)
		supportInput.RawValue = scalar(float64(count) / float64(total))
		supportInput.Sample.Numerator = &count
		supportInput.Sample.Denominator = &total
		if hasSource {
			_ = testInput.RawValue.UnmarshalJSON([]byte(strconv.FormatBool(len(tests) > 0)))
		} else {
			testInput.Status = "not_applicable"
			testInput.Reason = reason("NO_SOURCE_FILES", "В дереве нет файлов с поддерживаемыми расширениями исходного кода")
		}
	}
	return []domain.MetricInput{readmeInput, supportInput, testInput}
}

func sourceFile(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".java", ".c", ".h", ".cpp", ".hpp", ".cs", ".rb", ".rs":
		return true
	default:
		return false
	}
}

func testFile(dir, name string, isSource bool) bool {
	for _, pattern := range []string{"test_*.*", "*_test.*", "*.test.*", "*.spec.*"} {
		if matched, _ := path.Match(pattern, name); matched {
			return true
		}
	}
	if isSource {
		for part := range strings.SplitSeq(dir, "/") {
			switch part {
			case "tests", "test", "__tests__", "spec":
				return true
			}
		}
	}
	return false
}

func treeInput(id, unit string, tree domain.SourceState, revision *string, paths []string, limit int) domain.MetricInput {
	input := domain.MetricInput{ID: id, Unit: unit, Status: tree.Status, Reason: tree.Reason, Sample: tree.Sample,
		Series: []int{}, Evidence: []domain.Evidence{}, Exclusions: []domain.ExcludedCount{}}
	slices.Sort(paths)
	paths = slices.Compact(paths)
	for _, name := range paths {
		if len(input.Evidence) == limit {
			break
		}
		if !domain.ValidEvidencePath(name) {
			continue
		}
		input.Evidence = append(input.Evidence, domain.Evidence{
			ID: id + ":" + name, Source: "git", Kind: "file",
			Description: "Файл в дереве основной ветки", Path: &name, Revision: revision,
		})
	}
	input.EvidenceLimited = len(paths) > len(input.Evidence)
	if tree.Status == "complete" {
		input.EvidenceTotal = pointer(len(paths))
	}
	return input
}

// TreeCategories scores the documentation and test indicators from snapshot inputs.
func (e *Engine) TreeCategories(versions domain.Versions, inputs []domain.MetricInput) ([]domain.Category, error) {
	if err := e.checkVersions(versions); err != nil {
		return nil, err
	}
	byID := make(map[string]domain.MetricInput, len(inputs))
	for _, input := range inputs {
		if _, exists := byID[input.ID]; exists {
			return nil, invalid("duplicate snapshot input")
		}
		byID[input.ID] = input
	}
	categories := make([]domain.Category, 0, 2)
	for _, id := range []string{"documentation", "testing"} {
		for _, cfg := range e.config.Categories {
			if cfg.ID != id {
				continue
			}
			metrics := make([]domain.Metric, 0, len(cfg.Metrics))
			for _, spec := range cfg.Metrics {
				input, exists := byID[spec.ID]
				if !exists {
					return nil, invalid("missing tree input")
				}
				metric, err := e.treeMetric(input, spec)
				if err != nil {
					return nil, err
				}
				metrics = append(metrics, metric)
			}
			subject, explanation := "документации", "Оценка наличия базовых документов в дереве основной ветки."
			if id == "testing" {
				subject, explanation = "тестирования", testExplanation
			}
			categories = append(categories, summarizeCategory(cfg, metrics, subject, explanation))
			break
		}
	}
	return categories, nil
}

func (e *Engine) treeMetric(input domain.MetricInput, spec metricConfig) (domain.Metric, error) {
	if err := input.Validate(); err != nil {
		return domain.Metric{}, err
	}
	metric := domain.Metric{Input: input, RuleVersion: e.config.MethodologyVersion,
		Rule: "true → 100, false → 0", Explanation: "Балл за наличие файла в дереве основной ветки."}
	if spec.Normalization == "share" {
		metric.Rule = "100 × число найденных видов / число видов в методике"
		metric.Explanation = "Каждый вид сопровождающего документа учитывается один раз."
	}
	if input.ID == "TEST_PRESENCE" {
		metric.Explanation = testExplanation
	}
	if threshold, exists := e.config.FindingThresholds[spec.ID]; exists {
		metric.Threshold = &threshold
	}
	if input.Status != "complete" {
		return metric, nil
	}
	raw, _ := input.RawValue.MarshalJSON()
	score := 0.0
	switch spec.Normalization {
	case "boolean":
		var present *bool
		if input.Unit != "bool" || json.Unmarshal(raw, &present) != nil || present == nil {
			return domain.Metric{}, invalid("tree boolean raw_value")
		}
		if *present {
			score = 100
		}
	case "share":
		var share *float64
		count, total := input.Sample.Numerator, input.Sample.Denominator
		if input.Unit != "ratio" || json.Unmarshal(raw, &share) != nil || share == nil ||
			count == nil || total == nil || *total != len(e.config.SupportingDocuments) ||
			*count > *total || *share != float64(*count)/float64(*total) {
			return domain.Metric{}, invalid("documentation share sample/raw_value mismatch")
		}
		score = *share * 100
	}
	metric.NormalizedScore = &score
	return metric, nil
}
