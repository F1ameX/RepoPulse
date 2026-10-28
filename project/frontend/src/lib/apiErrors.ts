import { ApiError } from '@/services/api';

export const SAFE_MESSAGES: Record<string, string> = {
  ANALYSIS_FAILED: 'Анализ завершился ошибкой. Запустите его заново.',
  ANALYSIS_NOT_FOUND: 'Анализ не найден.',
  AUTH_UNAVAILABLE: 'Сервис сессий временно недоступен. Повторите позже.',
  REPORT_NOT_FOUND: 'Отчёт не найден.',
  REPORT_SAVE_FAILED: 'Не удалось сохранить отчёт.',
  REPOSITORY_NOT_FOUND_OR_PRIVATE: 'Публичный репозиторий недоступен.',
  VALIDATION_FAILED: 'Сервер отклонил параметры запроса.',
  SESSION_REQUIRED: 'Требуется вход в систему.',
  SESSION_EXPIRED: 'Сессия истекла. Войдите снова.',
  LOGIN_REQUIRED: 'Требуется войти в систему.',
  UPSTREAM_RATE_LIMITED: 'Сервис временно ограничил запросы. Повторите позже.',
  UPSTREAM_TIMEOUT: 'Сервис не ответил вовремя. Повторите позже.',
  NETWORK_ERROR: 'Нет соединения с сервером.',
  INTERNAL_ERROR: 'Сервис временно недоступен.',
  CSRF_REJECTED: 'Запрос отклонён. Обновите страницу.',
};

export function safeMessage(error: unknown, fallback = 'Не удалось выполнить операцию. Повторите позже.'): string {
  if (error instanceof ApiError) {
    return SAFE_MESSAGES[error.code] ?? fallback;
  }
  return fallback;
}