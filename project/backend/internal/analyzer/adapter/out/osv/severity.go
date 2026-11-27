package osv

import (
	"math"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type advisoryDetails struct {
	ID               string          `json:"id"`
	Severity         []severityScore `json:"severity"`
	DatabaseSpecific struct {
		Severity string `json:"severity"`
	} `json:"database_specific"`
	Affected []struct {
		Package struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
		} `json:"package"`
		Severity []severityScore `json:"severity"`
	} `json:"affected"`
}

func (a advisoryDetails) level(query domain.OSVQuery) string {
	level := advisorySeverity(a.DatabaseSpecific.Severity, a.Severity)
	for _, affected := range a.Affected {
		if affected.Package.Ecosystem == query.Ecosystem && affected.Package.Name == query.Name {
			if candidate := advisorySeverity("", affected.Severity); severityRank(candidate) > severityRank(level) {
				level = candidate
			}
		}
	}
	return level
}

type severityScore struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

func severityRank(level string) int {
	switch level {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func advisorySeverity(label string, scores []severityScore) string {
	level := strings.ToLower(strings.TrimSpace(label))
	if level == "moderate" {
		level = "medium"
	}
	if severityRank(level) == 0 {
		level = ""
	}
	for _, entry := range scores {
		if entry.Type != "CVSS_V3" {
			continue
		}
		score := cvss3Base(entry.Score)
		candidate := ""
		switch {
		case score >= 9:
			candidate = "critical"
		case score >= 7:
			candidate = "high"
		case score >= 4:
			candidate = "medium"
		case score > 0:
			candidate = "low"
		}
		if severityRank(candidate) > severityRank(level) {
			level = candidate
		}
	}
	return level
}

// cvss3Base implements the CVSS 3.0 and 3.1 base equations.
func cvss3Base(vector string) float64 {
	fields := strings.Split(vector, "/")
	if fields[0] != "CVSS:3.0" && fields[0] != "CVSS:3.1" {
		return 0
	}
	metrics := make(map[string]string)
	for _, field := range fields[1:] {
		key, value, ok := strings.Cut(field, ":")
		if !ok || value == "" || metrics[key] != "" {
			return 0
		}
		metrics[key] = value
	}
	scope := metrics["S"]
	av := map[string]float64{"N": .85, "A": .62, "L": .55, "P": .2}[metrics["AV"]]
	ac := map[string]float64{"L": .77, "H": .44}[metrics["AC"]]
	pr := map[string]float64{"N": .85, "L": .62, "H": .27}[metrics["PR"]]
	if scope == "C" {
		pr = map[string]float64{"N": .85, "L": .68, "H": .50}[metrics["PR"]]
	}
	ui := map[string]float64{"N": .85, "R": .62}[metrics["UI"]]
	impactWeights := map[string]float64{"N": 0, "L": .22, "H": .56}
	c, validC := impactWeights[metrics["C"]]
	i, validI := impactWeights[metrics["I"]]
	a, validA := impactWeights[metrics["A"]]
	if av == 0 || ac == 0 || pr == 0 || ui == 0 || !validC || !validI || !validA || (scope != "C" && scope != "U") {
		return 0
	}
	base := 1 - (1-c)*(1-i)*(1-a)
	impact := 6.42 * base
	multiplier := 1.0
	if scope == "C" {
		impact = 7.52*(base-.029) - 3.25*math.Pow(base-.02, 15)
		multiplier = 1.08
	}
	if impact <= 0 {
		return 0
	}
	rounded := math.Round(math.Min(10, multiplier*(impact+8.22*av*ac*pr*ui)) * 100000)
	return math.Ceil(rounded/10000) / 10
}
