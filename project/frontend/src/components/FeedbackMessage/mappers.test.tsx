import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { submissionFeedback, statusFeedback, reportFeedback } from './mappers';
import type { AnalysisStatusState } from '@/hooks/useAnalysisStatus';

afterEach(cleanup);

const noop = {
  retrySame: vi.fn(),
  retryNew: vi.fn(),
  onChangeUrl: vi.fn(),
  onOpen: vi.fn(),
  onRefresh: vi.fn(),
  onReload: vi.fn(),
};

describe('acceptance: temporary-error vs unknown vs confirmed-error are distinct', () => {
  it('unknown → warning "Не удалось подтвердить запуск анализа"', () => {
    const view = submissionFeedback(
      { kind: 'unknown', requestId: 'r', originalUrl: 'a', since: '' },
      'a',
      noop,
    );
    expect(view?.variant).toBe('warning');
    expect(view?.message).toContain('Не удалось подтвердить запуск анализа');
  });

  it('unknown + changed URL → hint about a possible previous acceptance', () => {
    const view = submissionFeedback(
      { kind: 'unknown', requestId: 'r', originalUrl: 'a', since: '' },
      'b',
      noop,
    );
    expect(view?.hint).toContain('прежний запрос мог быть уже принят');
  });

  it('temporary-error → warning with the safe message and retrySame', () => {
    const view = submissionFeedback(
      { kind: 'temporary-error', requestId: 'r', originalUrl: 'a', message: 'X' },
      'a',
      noop,
    );
    expect(view?.variant).toBe('warning');
    render(<>{view?.actions}</>);
    expect(screen.getByRole('button', { name: 'Повторить отправку' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Изменить URL' })).toBeTruthy();
  });

  it('confirmed-error → error with "Повторить анализ" and "Изменить URL"', () => {
    const view = submissionFeedback(
      {
        kind: 'confirmed-error',
        requestId: 'r',
        originalUrl: 'a',
        message: 'Y',
        code: 'ANALYSIS_FAILED',
      },
      'a',
      noop,
    );
    expect(view?.variant).toBe('error');
    render(<>{view?.actions}</>);
    expect(screen.getByRole('button', { name: 'Повторить анализ' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Изменить URL' })).toBeTruthy();
  });

  // NOTE: FR-03 P01 требует «Запускаем анализ» (D2, #18480).
  it('submitting currently renders nothing (D2)', () => {
    expect(
      submissionFeedback({ kind: 'submitting', requestId: 'r', originalUrl: 'a' }, 'a', noop),
    ).toBeNull();
  });
});

describe('statusFeedback', () => {
  it('S06/N02: error without a confirmed state → "Состояние пока неизвестно"', () => {
    const state: AnalysisStatusState = {
      kind: 'error',
      analysisId: 'a',
      message: 'Не удалось обновить статус',
      lastConfirmed: null,
    };
    expect(statusFeedback(state, noop)?.hint).toBe('Состояние пока неизвестно');
  });

  it.each(['SESSION_REQUIRED', 'SESSION_EXPIRED', 'LOGIN_REQUIRED'] as const)(
    'N09: hidden %s → session expired message',
    (code) => {
      expect(statusFeedback({ kind: 'hidden', analysisId: 'a', code }, noop)?.message).toBe(
        'Сессия истекла. Войдите снова',
      );
    },
  );

  it('hidden with a non-session code → "Анализ недоступен"', () => {
    expect(
      statusFeedback({ kind: 'hidden', analysisId: 'a', code: 'FORBIDDEN' }, noop)?.message,
    ).toBe('Анализ недоступен');
  });
});

describe('reportFeedback', () => {
  it('N07: error keeps a hint about previously loaded data', () => {
    const view = reportFeedback(
      {
        kind: 'error',
        reportId: 'r',
        message: 'Не удалось загрузить отчёт',
        data: {} as never,
        loadedAt: '2026-10-29T10:00:00Z',
      },
      noop,
    );
    expect(view?.hint).toContain('Показаны сохранённые данные');
    render(<>{view?.actions}</>);
    expect(screen.getByRole('button', { name: 'Повторить загрузку' })).toBeTruthy();
  });

  it('N07: loading with saved data hints that the previous data is shown', () => {
    const view = reportFeedback(
      { kind: 'loading', reportId: 'r', data: {} as never, loadedAt: '2026-10-29T10:00:00Z' },
      noop,
    );
    expect(view?.message).toBe('Обновляем отчёт…');
    expect(view?.hint).toContain('Показаны сохранённые данные');
  });
});