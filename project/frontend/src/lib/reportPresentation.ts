import type { Coverage } from '@/types';

export function isReportScore(value: unknown): value is number | null {
  return (
    value === null ||
    (typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 100)
  );
}

export function isReportCoverage(value: unknown, total: number): value is Coverage {
  if (!value || typeof value !== 'object') return false;
  const coverage = value as Coverage;
  return (
    coverage.total === total &&
    [coverage.calculated, coverage.partial, coverage.unavailable, coverage.not_applicable].every(
      (count) => Number.isInteger(count) && count >= 0 && count <= total,
    ) &&
    coverage.partial + coverage.unavailable + coverage.not_applicable <= total &&
    coverage.calculated + coverage.unavailable + coverage.not_applicable <= total &&
    typeof coverage.percentage === 'number' &&
    Number.isFinite(coverage.percentage) &&
    coverage.percentage >= 0 &&
    coverage.percentage <= 100
  );
}
