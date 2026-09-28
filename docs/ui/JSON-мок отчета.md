`report.json`:
```json
{
  "analysisId": "analysis_123qwe",
  "reportId": "report_123ewq",

  "status": "completed",
  "error": null,

  "repository": {
    "platform": "github",
    "url": "https://github.com/example",
    "owner": "example",
    "name": "example",
    "description": "An example web application.",
    "defaultBranch": "main",
    "commitSha": "3f7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b",
    "visibility": "public"
  },

  "analysis": {
    "startedAt": "2026-09-01T00:02:28Z",
    "completedAt": "2026-09-01T00:03:28Z",
    "durationSeconds": 60,
    "schemaVersion": "1.0.0",
    "methodology": {
      "id": "repo-health-default",
      "version": "1.0.0",
      "name": "RepoPulse Standard"
    }
  },

  "score": {
    "value": 78,
    "label": "Good",
    "summary": "The repository is in good overall condition, with strong documentation, versioning and code review practices. The main risks are outdated dependencies and incomplete CI/CD data.",
    "keyFactors": [
      {
        "type": "positive",
        "categoryId": "documentation",
        "text": "The repository contains a clear README, contribution guidelines and architecture documentation."
      },
      {
        "type": "positive",
        "categoryId": "code_review",
        "text": "Most recent pull requests have at least one completed review before merging."
      },
      {
        "type": "negative",
        "categoryId": "dependencies",
        "text": "Several production dependencies are outdated and one dependency has a known security advisory."
      },
      {
        "type": "negative",
        "categoryId": "ci_cd",
        "text": "CI/CD results could not be fully retrieved from the platform API."
      }
    ],
    "dataCompleteness": {
      "status": "partial",
      "percentage": 91,
      "summary": "Most repository and Git data was available. CI/CD data was partially unavailable because the platform API did not provide complete workflow results.",
      "limitationsCount": 1
    }
  },

  "categories": [
    {
      "id": "change_history",
      "name": "Change History",
      "score": 84,
      "weight": 0.12,
      "status": "available",
      "summary": "The repository has regular development activity and a generally healthy commit history.",
      "positiveObservations": [
        {
          "id": "CH-POS-001",
          "text": "The repository has received commits during the last 30 days."
        },
        {
          "id": "CH-POS-002",
          "text": "Changes are distributed across multiple contributors."
        },
        {
          "id": "CH-POS-003",
          "text": "There are no unusually large periods of inactivity in the recent history."
        }
      ],
      "negativeObservations": [
        {
          "id": "CH-NEG-001",
          "text": "Several commits contain multiple unrelated changes."
        }
      ],
      "evidence": [
        {
          "type": "metric",
          "name": "Commits in last 30 days",
          "value": 47,
          "unit": "commits",
          "source": "git-history"
        },
        {
          "type": "metric",
          "name": "Active contributors in last 90 days",
          "value": 5,
          "unit": "contributors",
          "source": "git-history"
        },
        {
          "type": "metric",
          "name": "Median commits per week",
          "value": 11,
          "unit": "commits/week",
          "source": "git-history"
        },
        {
          "type": "repository",
          "name": "Analyzed branch",
          "value": "main",
          "source": "git"
        }
      ]
    },

    {
      "id": "documentation",
      "name": "Documentation",
      "score": 94,
      "weight": 0.1,
      "status": "available",
      "summary": "Documentation coverage is strong and provides enough information for a new contributor to understand the project.",
      "positiveObservations": [
        {
          "id": "DOC-POS-001",
          "text": "README contains project purpose, setup instructions and usage information."
        },
        {
          "id": "DOC-POS-002",
          "text": "Contribution guidelines are available."
        },
        {
          "id": "DOC-POS-003",
          "text": "Architecture documentation is present."
        }
      ],
      "negativeObservations": [
        {
          "id": "DOC-NEG-001",
          "text": "API documentation does not cover all public endpoints."
        }
      ],
      "evidence": [
        {
          "type": "file",
          "name": "README",
          "path": "README.md",
          "value": "present"
        },
        {
          "type": "file",
          "name": "Contribution guidelines",
          "path": "CONTRIBUTING.md",
          "value": "present"
        },
        {
          "type": "file",
          "name": "Architecture documentation",
          "path": "docs/architecture.md",
          "value": "present"
        },
        {
          "type": "file",
          "name": "API documentation",
          "path": "docs/api.md",
          "value": "partial"
        }
      ]
    },

    {
      "id": "tests",
      "name": "Tests",
      "score": 76,
      "weight": 0.12,
      "status": "available",
      "summary": "The project has an established automated test suite, but coverage is uneven across modules.",
      "positiveObservations": [
        {
          "id": "TEST-POS-001",
          "text": "Automated unit tests are present."
        },
        {
          "id": "TEST-POS-002",
          "text": "Integration tests cover the main API flows."
        }
      ],
      "negativeObservations": [
        {
          "id": "TEST-NEG-001",
          "text": "Several service-layer modules have low test coverage."
        },
        {
          "id": "TEST-NEG-002",
          "text": "There are no end-to-end tests for the main user flow."
        }
      ],
      "evidence": [
        {
          "type": "metric",
          "name": "Line coverage",
          "value": 71,
          "unit": "percent",
          "source": "coverage-report"
        },
        {
          "type": "metric",
          "name": "Unit test files",
          "value": 38,
          "unit": "files",
          "source": "repository"
        },
        {
          "type": "metric",
          "name": "Integration test files",
          "value": 12,
          "unit": "files",
          "source": "repository"
        },
        {
          "type": "file",
          "name": "Test directory",
          "path": "src/test",
          "value": "present"
        }
      ]
    },

    {
      "id": "dependencies",
      "name": "Dependencies",
      "score": 58,
      "weight": 0.12,
      "status": "available",
      "summary": "Dependency management is configured, but several dependencies are outdated and require attention.",
      "positiveObservations": [
        {
          "id": "DEP-POS-001",
          "text": "Dependencies are declared using the project's standard package manager."
        },
        {
          "id": "DEP-POS-002",
          "text": "Dependency lockfile is committed to the repository."
        }
      ],
      "negativeObservations": [
        {
          "id": "DEP-NEG-001",
          "text": "Seven production dependencies have newer stable versions available."
        },
        {
          "id": "DEP-NEG-002",
          "text": "One production dependency is affected by a known security advisory."
        }
      ],
      "evidence": [
        {
          "type": "file",
          "name": "Package manifest",
          "path": "package.json",
          "value": "present"
        },
        {
          "type": "file",
          "name": "Lockfile",
          "path": "package-lock.json",
          "value": "present"
        },
        {
          "type": "metric",
          "name": "Outdated production dependencies",
          "value": 7,
          "unit": "dependencies",
          "source": "dependency-analysis"
        },
        {
          "type": "metric",
          "name": "Known security advisories",
          "value": 1,
          "unit": "advisories",
          "source": "dependency-analysis"
        }
      ]
    },

    {
      "id": "ci_cd",
      "name": "CI/CD",
      "score": null,
      "weight": 0.12,
      "status": "unavailable",
      "summary": "CI/CD data could not be fully retrieved. The repository configuration was detected, but recent workflow results are unavailable.",
      "positiveObservations": [
        {
          "id": "CICD-POS-001",
          "text": "CI workflow configuration is present in the repository."
        }
      ],
      "negativeObservations": [],
      "unavailableReason": "The GitHub Actions API did not return complete workflow run history for the analyzed repository.",
      "evidence": [
        {
          "type": "file",
          "name": "GitHub Actions workflow",
          "path": ".github/workflows/ci.yml",
          "value": "present"
        },
        {
          "type": "api",
          "name": "Workflow run history",
          "value": "unavailable",
          "source": "github-actions-api"
        }
      ]
    },

    {
      "id": "security",
      "name": "Security",
      "score": 72,
      "weight": 0.14,
      "status": "available",
      "summary": "No critical repository-level security issues were detected, but dependency security and secret-management practices require attention.",
      "positiveObservations": [
        {
          "id": "SEC-POS-001",
          "text": "No committed private keys or obvious credentials were detected."
        },
        {
          "id": "SEC-POS-002",
          "text": "Security-related configuration is separated from application source code."
        }
      ],
      "negativeObservations": [
        {
          "id": "SEC-NEG-001",
          "text": "A production dependency has a known security advisory."
        },
        {
          "id": "SEC-NEG-002",
          "text": "The repository does not contain a documented security policy."
        }
      ],
      "evidence": [
        {
          "type": "metric",
          "name": "Critical secret findings",
          "value": 0,
          "unit": "findings",
          "source": "secret-scan"
        },
        {
          "type": "metric",
          "name": "High severity secret findings",
          "value": 0,
          "unit": "findings",
          "source": "secret-scan"
        },
        {
          "type": "metric",
          "name": "Dependency security advisories",
          "value": 1,
          "unit": "advisories",
          "source": "dependency-analysis"
        },
        {
          "type": "file",
          "name": "Security policy",
          "path": "SECURITY.md",
          "value": "missing"
        }
      ]
    },

    {
      "id": "versioning",
      "name": "Versioning",
      "score": 91,
      "weight": 0.08,
      "status": "available",
      "summary": "The project follows a consistent release and versioning approach.",
      "positiveObservations": [
        {
          "id": "VER-POS-001",
          "text": "Release tags are present."
        },
        {
          "id": "VER-POS-002",
          "text": "The project declares an explicit application version."
        }
      ],
      "negativeObservations": [
        {
          "id": "VER-NEG-001",
          "text": "Release notes are missing for two recent versions."
        }
      ],
      "evidence": [
        {
          "type": "metric",
          "name": "Releases in last 12 months",
          "value": 8,
          "unit": "releases",
          "source": "github-releases"
        },
        {
          "type": "metric",
          "name": "Version tags",
          "value": 8,
          "unit": "tags",
          "source": "git-tags"
        },
        {
          "type": "file",
          "name": "Application version",
          "path": "package.json",
          "value": "2.4.1"
        }
      ]
    },

    {
      "id": "pull_merge_requests",
      "name": "Pull / Merge Requests",
      "score": 81,
      "weight": 0.1,
      "status": "available",
      "summary": "Changes are usually introduced through pull requests with a consistent review workflow.",
      "positiveObservations": [
        {
          "id": "PR-POS-001",
          "text": "Most recent changes were merged through pull requests."
        },
        {
          "id": "PR-POS-002",
          "text": "Pull requests usually contain descriptive titles."
        }
      ],
      "negativeObservations": [
        {
          "id": "PR-NEG-001",
          "text": "Several pull requests contain very large changesets."
        }
      ],
      "evidence": [
        {
          "type": "metric",
          "name": "Pull requests in last 90 days",
          "value": 34,
          "unit": "pull requests",
          "source": "github-pull-requests"
        },
        {
          "type": "metric",
          "name": "Merged through pull request",
          "value": 89,
          "unit": "percent",
          "source": "github-pull-requests"
        },
        {
          "type": "metric",
          "name": "Median files changed per pull request",
          "value": 8,
          "unit": "files",
          "source": "github-pull-requests"
        }
      ]
    },

    {
      "id": "code_review",
      "name": "Code Review",
      "score": 88,
      "weight": 0.1,
      "status": "available",
      "summary": "Code review is consistently used and most changes receive review before being merged.",
      "positiveObservations": [
        {
          "id": "REVIEW-POS-001",
          "text": "Most merged pull requests have at least one approved review."
        },
        {
          "id": "REVIEW-POS-002",
          "text": "Review comments are present on non-trivial changes."
        }
      ],
      "negativeObservations": [
        {
          "id": "REVIEW-NEG-001",
          "text": "Some small pull requests were merged without an explicit approval."
        }
      ],
      "evidence": [
        {
          "type": "metric",
          "name": "Pull requests with at least one approval",
          "value": 93,
          "unit": "percent",
          "source": "github-reviews"
        },
        {
          "type": "metric",
          "name": "Pull requests without approval",
          "value": 7,
          "unit": "percent",
          "source": "github-reviews"
        },
        {
          "type": "metric",
          "name": "Median reviewers per pull request",
          "value": 2,
          "unit": "reviewers",
          "source": "github-reviews"
        }
      ]
    }
  ],

  "issues": [
    {
      "id": "SEC-001",
      "categoryId": "security",
      "title": "Dependency with known security advisory",
      "description": "One production dependency is affected by a known security advisory and should be updated to a non-affected version.",
      "severity": "critical",
      "evidence": [
        {
          "type": "dependency",
          "name": "example-http-client",
          "value": "3.2.1",
          "expected": "3.4.0 or later"
        },
        {
          "type": "advisory",
          "name": "Security advisory",
          "value": "GHSA-example-1234",
          "source": "GitHub Security Advisories"
        }
      ],
      "relatedPath": "package-lock.json",
      "relatedUrl": "https://github.com/example/security/dependabot"
    },

    {
      "id": "DEP-001",
      "categoryId": "dependencies",
      "title": "Outdated production dependencies",
      "description": "Seven production dependencies have newer stable versions available. Keeping dependencies outdated may increase compatibility and security risks.",
      "severity": "high",
      "evidence": [
        {
          "type": "metric",
          "name": "Outdated production dependencies",
          "value": 7,
          "unit": "dependencies"
        }
      ],
      "relatedPath": "package.json"
    },

    {
      "id": "TEST-001",
      "categoryId": "tests",
      "title": "Uneven test coverage",
      "description": "Overall line coverage is 71%, but several service modules have substantially lower coverage than the project average.",
      "severity": "medium",
      "evidence": [
        {
          "type": "metric",
          "name": "Line coverage",
          "value": 71,
          "unit": "percent"
        }
      ],
      "relatedPath": "src/main/services"
    },

    {
      "id": "DOC-001",
      "categoryId": "documentation",
      "title": "Incomplete API documentation",
      "description": "The API documentation exists but does not describe all public endpoints exposed by the application.",
      "severity": "medium",
      "evidence": [
        {
          "type": "file",
          "name": "API documentation",
          "path": "docs/api.md",
          "value": "partial"
        },
        {
          "type": "metric",
          "name": "Documented public endpoints",
          "value": 14,
          "unit": "endpoints"
        },
        {
          "type": "metric",
          "name": "Detected public endpoints",
          "value": 19,
          "unit": "endpoints"
        }
      ],
      "relatedPath": "docs/api.md"
    },

    {
      "id": "PR-001",
      "categoryId": "pull_merge_requests",
      "title": "Large pull requests",
      "description": "Several pull requests contain large changesets, which can make review and troubleshooting more difficult.",
      "severity": "low",
      "evidence": [
        {
          "type": "metric",
          "name": "Large pull requests in last 90 days",
          "value": 4,
          "unit": "pull requests",
          "threshold": 20
        }
      ],
      "relatedUrl": "https://github.com/example/pulls"
    },

    {
      "id": "VER-001",
      "categoryId": "versioning",
      "title": "Missing release notes",
      "description": "Two recent releases do not contain release notes describing the changes introduced in the corresponding versions.",
      "severity": "info",
      "evidence": [
        {
          "type": "metric",
          "name": "Releases without release notes",
          "value": 2,
          "unit": "releases"
        }
      ],
      "relatedUrl": "https://github.com/example/releases"
    }
  ],

  "recommendations": [
    {
      "id": "REC-SEC-001",
      "priority": "critical",
      "action": "Update the affected production dependency to a version that is not affected by the reported security advisory.",
      "expectedEffect": "Reduce the known security risk associated with the affected dependency.",
      "relatedIssueIds": [
        "SEC-001"
      ],
      "effort": "small"
    },

    {
      "id": "REC-DEP-001",
      "priority": "high",
      "action": "Review and update the seven outdated production dependencies, starting with dependencies that have security or compatibility implications.",
      "expectedEffect": "Reduce dependency-related security and compatibility risks and keep the project closer to supported versions.",
      "relatedIssueIds": [
        "DEP-001",
        "SEC-001"
      ],
      "effort": "medium"
    },

    {
      "id": "REC-TEST-001",
      "priority": "medium",
      "action": "Add automated tests for service modules with coverage below the project average and introduce end-to-end tests for the main user flow.",
      "expectedEffect": "Increase confidence in changes and reduce the probability of regressions.",
      "relatedIssueIds": [
        "TEST-001"
      ],
      "effort": "large"
    },

    {
      "id": "REC-DOC-001",
      "priority": "medium",
      "action": "Document the five public API endpoints that are currently missing from the API documentation.",
      "expectedEffect": "Make the API easier to understand and reduce onboarding time for contributors.",
      "relatedIssueIds": [
        "DOC-001"
      ],
      "effort": "small"
    },

    {
      "id": "REC-PR-001",
      "priority": "low",
      "action": "Split large pull requests into smaller logically independent changes where practical.",
      "expectedEffect": "Make code review easier and improve the ability to identify problems introduced by individual changes.",
      "relatedIssueIds": [
        "PR-001"
      ],
      "effort": "medium"
    },

    {
      "id": "REC-VER-001",
      "priority": "low",
      "action": "Add release notes to the two recent releases that currently do not contain them.",
      "expectedEffect": "Make version changes easier to understand for users and contributors.",
      "relatedIssueIds": [
        "VER-001"
      ],
      "effort": "small"
    }
  ],

  "limitations": [
    {
      "id": "LIMIT-001",
      "type": "unavailable_data",
      "categoryId": "ci_cd",
      "title": "Incomplete CI/CD data",
      "description": "The GitHub Actions API did not return complete workflow run history for the analyzed repository. CI/CD score was therefore not calculated.",
      "source": "github-actions-api",
      "status": "unavailable"
    }
  ]
}
```