package snapshot

import (
	"reflect"
	"sort"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type dependencyKey struct{ ecosystem, name, version string }

func dependencies(items []domain.Dependency) ([]domain.Dependency, error) {
	byKey := map[dependencyKey]domain.Dependency{}
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return nil, err
		}
		key := dependencyKey{item.Ecosystem, item.Package, item.Version}
		if previous, found := byKey[key]; found {
			item.Origins = append(previous.Origins, item.Origins...)
		}
		seen := map[domain.DependencyOrigin]bool{}
		origins := []domain.DependencyOrigin{}
		for _, origin := range item.Origins {
			if !seen[origin] {
				seen[origin] = true
				origins = append(origins, origin)
			}
		}
		sort.Slice(origins, func(i, j int) bool {
			a, b := origins[i], origins[j]
			return a.ManifestPath < b.ManifestPath || a.ManifestPath == b.ManifestPath && a.VersionSource < b.VersionSource
		})
		item.Origins = origins
		item.ManifestPath, item.VersionSource = origins[0].ManifestPath, origins[0].VersionSource
		byKey[key] = item
	}
	result := make([]domain.Dependency, 0, len(byKey))
	for _, item := range byKey {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Ecosystem != b.Ecosystem {
			return a.Ecosystem < b.Ecosystem
		}
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		return a.Version < b.Version
	})
	return result, nil
}

func vulnerabilities(items []domain.Vulnerability) ([]domain.Vulnerability, error) {
	type key struct {
		dependencyKey
		id string
	}
	byKey := map[key]domain.Vulnerability{}
	for _, item := range items {
		if item.ObtainedAt.IsZero() {
			return nil, domain.ErrInvalidData
		}
		k := key{dependencyKey{item.Ecosystem, item.Package, item.Version}, item.ID}
		if previous, found := byKey[k]; found {
			if !reflect.DeepEqual(previous.Severity, item.Severity) || !reflect.DeepEqual(previous.EvidenceURL, item.EvidenceURL) {
				return nil, domain.ErrInvalidData
			}
			if previous.ObtainedAt.After(item.ObtainedAt) {
				item = previous
			}
		}
		byKey[k] = item
	}
	result := make([]domain.Vulnerability, 0, len(byKey))
	for _, item := range byKey {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Ecosystem != b.Ecosystem {
			return a.Ecosystem < b.Ecosystem
		}
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		return a.Version < b.Version
	})
	return result, nil
}
