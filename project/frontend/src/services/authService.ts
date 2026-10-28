import api from './api';

export interface SessionInfo {
  kind: 'account' | 'guest';
  login: string | null;
  expires_at: string;
}

export interface SessionContext {
  account: SessionInfo | null;
  guest: SessionInfo | null;
}

export const authService = {
  async createGuestSession(signal?: AbortSignal): Promise<SessionInfo> {
    const { data } = await api.post<SessionInfo>('/auth/guest-session', undefined, { signal });
    return data;
  },

  async getSessionContext(signal?: AbortSignal): Promise<SessionContext> {
    const { data } = await api.get<SessionContext>('/auth/session', { signal });
    return data;
  },

  async logout(): Promise<void> {
    await api.post('/auth/logout');
  },
};
