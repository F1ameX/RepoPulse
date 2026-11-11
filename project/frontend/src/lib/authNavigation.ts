const REPORT_PATH = /^\/report\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function reportReturnTo(value: unknown): string | undefined {
  return typeof value === 'string' && value.length === 44 && REPORT_PATH.test(value)
    ? value.toLowerCase()
    : undefined;
}

export const SESSION_CHANGED = 'repopulse:session-changed';
export const SESSION_REVISION = 'repopulse.session-revision';
export const LOGOUT_PENDING = 'repopulse.logout-pending';
export const COOKIE_REQUIRED = 'Для работы RepoPulse разрешите cookie для этого сайта';

export function notifySessionChanged() {
  window.dispatchEvent(new Event(SESSION_CHANGED));
  try {
    localStorage.setItem(SESSION_REVISION, crypto.randomUUID());
  } catch {
    return;
  }
}
