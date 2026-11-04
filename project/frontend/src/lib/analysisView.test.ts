import { expect, it } from 'vitest';
import { analysisFailure } from './analysisView';

it.each([
  ['REPOSITORY_NOT_FOUND_OR_PRIVATE', 'internal detail', 'Репозиторий стал недоступен для анализа'],
  ['BUDGET_EXCEEDED', 'internal detail', 'Превышено время анализа'],
  [
    'ANALYSIS_FAILED',
    'Репозиторий превышает ограничения анализа',
    'Репозиторий превышает ограничения анализа',
  ],
  [
    'ANALYSIS_FAILED',
    'Не удалось получить данные для анализа',
    'Не удалось получить данные для анализа',
  ],
  ['REPORT_SAVE_FAILED', 'internal detail', 'Не удалось сохранить результат анализа'],
  ['ANALYSIS_FAILED', 'internal detail', 'Не удалось завершить анализ'],
  ['UNKNOWN', 'internal detail', 'Не удалось завершить анализ'],
  [null, undefined, 'Не удалось завершить анализ'],
])('%s shows a safe confirmed reason', (code, message, expected) => {
  expect(analysisFailure(code ?? null, message ?? undefined)).toBe(expected);
});
