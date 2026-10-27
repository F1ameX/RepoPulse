import { Button } from '@/components/Button/Button';
import type { FeedbackView } from './FeedbackMessage';
import type { SubmissionState } from '@/hooks/useAnalysisSubmission';
import type { AnalysisStatusState } from '@/hooks/useAnalysisStatus';
import type { ReportState } from '@/hooks/useReport';

const SAFE_REASONS: Record<string, string> = {
  ANALYSIS_FAILED: 'Анализ завершился ошибкой. Запустите его заново.',
  ANALYSIS_NOT_FOUND: 'Анализ не найден.',
  REPORT_NOT_FOUND: 'Отчёт не найден.',
  REPORT_SAVE_FAILED: 'Не удалось сохранить отчёт.',
  REPOSITORY_NOT_FOUND_OR_PRIVATE: 'Репозиторий недоступен.',
  VALIDATION_FAILED: 'Сервер отклонил параметры запроса.',
  SESSION_REQUIRED: 'Требуется вход в систему.',
  SESSION_EXPIRED: 'Сессия истекла. Войдите снова.',
  UPSTREAM_RATE_LIMITED: 'Сервис временно ограничил запросы. Повторите позже.',
  UPSTREAM_TIMEOUT: 'Сервис не ответил вовремя. Повторите позже.',
  NETWORK_ERROR: 'Нет соединения с сервером.',
  INTERNAL_ERROR: 'Сервис временно недоступен.',
};

function safeReason(code: string | null | undefined): string {
  if (!code) return 'Не удалось выполнить операцию. Повторите позже.';
  return SAFE_REASONS[code] ?? 'Не удалось выполнить операцию. Повторите позже.';
}

export interface SubmissionActions {
  retrySame: () => void;
  retryNew: () => void;
  onChangeUrl: () => void;
  onOpen: (analysisId: string) => void;
}

export function submissionFeedback(
  state: SubmissionState,
  currentUrl: string | undefined,
  actions: SubmissionActions,
): FeedbackView | null {
  switch (state.kind) {
    case 'idle':
      return null;

    case 'submitting':
      return {
        variant: 'info',
        icon: false,
        message: 'Отправляем запрос на анализ…',
      };

    case 'unknown':
    case 'temporary-error': {
      const urlChanged =
        currentUrl !== undefined && currentUrl !== state.originalUrl;
      return {
        variant: 'warning',
        message:
          state.kind === 'unknown'
            ? 'Не удалось подтвердить запуск анализа. Запрос мог быть принят.'
            : state.message,
        hint: urlChanged
          ? 'Вы изменили URL. Если отправите новый — это будет отдельный анализ; прежний запрос мог быть уже принят.'
          : undefined,
        actions: (
          <>
            <Button variant="secondary" size="sm" onClick={actions.retrySame}>
              Повторить отправку
            </Button>
            <Button variant="ghost" size="sm" onClick={actions.onChangeUrl}>
              Изменить URL
            </Button>
          </>
        ),
      };
    }

    case 'confirmed-error':
      return {
        variant: 'error',
        message: state.message,
        actions: (
          <>
            <Button variant="secondary" size="sm" onClick={actions.retryNew}>
              Повторить анализ
            </Button>
            <Button variant="ghost" size="sm" onClick={actions.onChangeUrl}>
              Изменить URL
            </Button>
          </>
        ),
      };

    case 'accepted':
      return {
        variant: 'success',
        message: 'Анализ запущен.',
        actions: (
          <Button
            variant="secondary"
            size="sm"
            onClick={() => actions.onOpen(state.analysisId)}
          >
            Открыть анализ
          </Button>
        ),
      };
  }
}

const STATUS_LABELS: Record<string, string> = {
  queued: 'В очереди',
  running: 'Выполняется',
  completed: 'Завершён',
  failed: 'Ошибка',
};

const STATUS_VARIANT: Record<string, FeedbackView['variant']> = {
  queued: 'info',
  running: 'info',
  completed: 'success',
  failed: 'error',
};

function formatUtc(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toISOString().replace('T', ' ').slice(0, 19) + ' UTC';
}

function lastConfirmedLine(
  status: string,
  updatedAt: string,
): string {
  return `Последнее подтверждённое: ${STATUS_LABELS[status] ?? status} (${formatUtc(updatedAt)})`;
}

export interface StatusActions {
  onRefresh: () => void;
}

export function statusFeedback(
  state: AnalysisStatusState,
  actions: StatusActions,
): FeedbackView | null {
  switch (state.kind) {
    case 'idle':
      return null;

    case 'loading':
      return {
        variant: 'info',
        icon: false,
        message: 'Проверяем статус анализа…',
        hint: state.lastConfirmed
          ? lastConfirmedLine(state.lastConfirmed.status, state.lastConfirmed.updatedAt)
          : undefined,
      };

    case 'confirmed': {
      const { status, reasonCode, updatedAt } = state.lastConfirmed;
      if (status === 'failed') {
        return {
          variant: 'error',
          message: reasonCode ? safeReason(reasonCode) : 'Анализ завершился ошибкой.',
          hint: `Подтверждено: ${formatUtc(updatedAt)}`,
        };
      }
      return {
        variant: STATUS_VARIANT[status] ?? 'info',
        message: `Статус: ${STATUS_LABELS[status] ?? status}`,
        hint: `Подтверждено: ${formatUtc(updatedAt)}`,
      };
    }

    case 'error':
      return {
        variant: 'warning',
        message: state.message,
        hint: state.lastConfirmed
          ? lastConfirmedLine(state.lastConfirmed.status, state.lastConfirmed.updatedAt)
          : 'Подтверждённых данных нет.',
        actions: (
          <Button variant="secondary" size="sm" onClick={actions.onRefresh}>
            Проверить статус
          </Button>
        ),
      };

    case 'offline':
      return {
        variant: 'warning',
        message: 'Нет соединения с сервером. Чтение приостановлено.',
        hint: state.lastConfirmed
          ? lastConfirmedLine(state.lastConfirmed.status, state.lastConfirmed.updatedAt)
          : undefined,
        actions: (
          <Button variant="secondary" size="sm" onClick={actions.onRefresh}>
            Проверить снова
          </Button>
        ),
      };
  }
}

export interface ReportActions {
  onReload: () => void;
}

export function reportFeedback(
  state: ReportState,
  actions: ReportActions,
): FeedbackView | null {
  switch (state.kind) {
    case 'idle':
    case 'loaded':
      return null;

    case 'loading':
      return {
        variant: 'info',
        icon: false,
        message: 'Загружаем отчёт…',
      };

    case 'offline':
      return {
        variant: 'warning',
        message: 'Нет соединения с сервером. Загрузка приостановлена.',
        actions: (
          <Button variant="secondary" size="sm" onClick={actions.onReload}>
            Повторить загрузку
          </Button>
        ),
      };

    case 'error':
      return {
        variant: 'warning',
        message: state.message,
        actions: (
          <Button variant="secondary" size="sm" onClick={actions.onReload}>
            Повторить загрузку
          </Button>
        ),
      };

    case 'hidden':
      return {
        variant: 'error',
        message: 'Данные отчёта недоступны.',
      };
  }
}