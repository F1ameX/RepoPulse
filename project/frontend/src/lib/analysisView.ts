import type { AnalysisStatusValue } from '@/types';

export const STATUS_LABELS: Record<AnalysisStatusValue, string> = {
  queued: 'Ожидание',
  running: 'Выполняется',
  completed: 'Завершён',
  failed: 'Ошибка',
};
export const STAGE_LABELS: Record<string, string> = {
  collecting_git: 'Получаем данные репозитория',
  collecting_external: 'Получаем данные источников',
  calculating: 'Рассчитываем показатели',
  explaining: 'Готовим объяснения',
  saving: 'Сохраняем отчёт',
};
export function formatUtc(value: string | null | undefined): string {
  if (!value) return 'Не определено';
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return 'Не определено';
  const iso = date.toISOString();
  return `${iso.slice(8, 10)}.${iso.slice(5, 7)}.${iso.slice(0, 4)} ${iso.slice(11, 19)} UTC`;
}
export function analysisFailure(code: string | null): string {
  const messages: Record<string, string> = {
    REPOSITORY_NOT_FOUND_OR_PRIVATE: 'Репозиторий стал недоступен для анализа',
    REPORT_SAVE_FAILED: 'Не удалось сохранить результат анализа',
  };
  return (code && messages[code]) || 'Не удалось завершить анализ';
}
export const isSessionError = (code: string) =>
  ['SESSION_REQUIRED', 'SESSION_EXPIRED', 'LOGIN_REQUIRED'].includes(code);
