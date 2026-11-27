package ci

import (
	"bytes"
	"context"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"gopkg.in/yaml.v3"
)

var semgrepCommand = regexp.MustCompile(`^(?:semgrep|semgrep.exe)[ \t]+(?:ci|scan)(?:[ \t]+[a-zA-Z0-9_./=:-]+)*[ \t]*$`)

// SASTInput collects configuration evidence from the pinned Git tree.
func SASTInput(ctx context.Context, session worker.GitSession, tree domain.SourceState) snapshot.Collection {
	git := session.Data()
	source := domain.SourceState{Source: "git", Scope: "sast_workflows", Status: tree.Status, Reason: tree.Reason, ObtainedAt: tree.ObtainedAt}
	input := metricInput("SEC_SAST", "bool", source)
	setStatus := func(status, code, message string) {
		source.Status = status
		source.Reason = &domain.Reason{Code: code, Message: message, CannotConclude: "Нельзя сделать полный вывод о настройке SAST"}
	}
	total, read, matches := 0, 0, 0
	otherCI, unsupported, unreadable := false, false, false
	if tree.Status == "complete" || tree.Status == "partial" {
		for _, file := range git.Files {
			switch file.Path {
			case ".gitlab-ci.yml", "Jenkinsfile", ".circleci/config.yml", "azure-pipelines.yml":
				otherCI = true
			}
			dir, name := path.Split(file.Path)
			if dir != ".github/workflows/" || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
				continue
			}
			total++
			if (file.Mode != "100644" && file.Mode != "100755") || !domain.ValidEvidencePath(file.Path) {
				unsupported = true
				continue
			}
			body, err := session.ReadBlob(ctx, file.ObjectSHA)
			if err != nil {
				unreadable = true
				continue
			}
			found, opaque, err := inspectSASTWorkflow(body)
			if err != nil {
				unreadable = true
				continue
			}
			read++
			unsupported = unsupported || opaque
			if found {
				matches++
				if len(input.Evidence) < evidenceLimit {
					input.Evidence = append(input.Evidence, domain.Evidence{
						ID: "SEC_SAST:" + file.Path, Source: "git", Kind: "file",
						Path: &file.Path, Revision: git.Repository.CommitSHA,
						Description: "Распознана настройка CodeQL или Semgrep в GitHub Actions",
					})
				}
			}
		}
	}
	if tree.Status == "complete" {
		source.Sample = domain.Sample{Collected: &read, Total: &total}
		switch {
		case unreadable:
			setStatus("partial", "SAST_WORKFLOW_UNREADABLE", "Не удалось прочитать или разобрать все workflow GitHub Actions")
		case unsupported || (total == 0 && otherCI):
			setStatus("not_applicable", "SAST_CONFIGURATION_UNSUPPORTED", "Формат конфигурации CI не поддерживается проверкой SAST")
		default:
			input.RawValue = scalar(strconv.FormatBool(matches > 0))
		}
	}
	input.Status, input.Reason, input.Sample = source.Status, source.Reason, source.Sample
	input.EvidenceTotal = &matches
	input.EvidenceLimited = matches > len(input.Evidence)
	key := snapshot.Source{Name: source.Source, Scope: source.Scope}
	return snapshot.Collection{Sources: []domain.SourceState{source}, Inputs: []snapshot.Input{{Value: input, Sources: []snapshot.Source{key}}}}
}

func inspectSASTWorkflow(body []byte) (found, unsupported bool, err error) {
	var workflow struct {
		Jobs map[string]struct {
			Uses  string
			Steps []struct{ Uses, Run string }
		}
	}
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&workflow); err != nil {
		return false, false, err
	}
	if err := decoder.Decode(new(yaml.Node)); err != io.EOF || len(workflow.Jobs) == 0 {
		return false, true, err
	}
	for _, job := range workflow.Jobs {
		unsupported = unsupported || job.Uses != ""
		for _, step := range job.Steps {
			action, version, pinned := strings.Cut(step.Uses, "@")
			if pinned && version != "" && strings.EqualFold(action, "github/codeql-action/analyze") {
				found = true
			}
			if semgrepCommand.MatchString(strings.TrimSpace(step.Run)) {
				found = true
			}
		}
	}
	return found, unsupported, nil
}
