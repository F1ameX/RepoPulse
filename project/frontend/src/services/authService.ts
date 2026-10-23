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
  async createGuestSession(): Promise<SessionInfo> {
    const { data } = await api.post<SessionInfo>('/auth/guest-session');
    return data;
  },

  async getSessionContext(): Promise<SessionContext> {
    const { data } = await api.get<SessionContext>('/auth/session');
    return data;
  },

  async logout(): Promise<void> {
    await api.post('/auth/logout');
  },
};
