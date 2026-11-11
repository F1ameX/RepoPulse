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

interface Credentials {
  login: string;
  password: string;
}

interface RegistrationResult {
  login: string;
  message: string;
}

export const authService = {
  async register(credentials: Credentials, signal?: AbortSignal): Promise<RegistrationResult> {
    const { data } = await api.post<RegistrationResult>('/auth/register', credentials, { signal });
    return data;
  },

  async login(credentials: Credentials, signal?: AbortSignal): Promise<SessionInfo> {
    const { data } = await api.post<SessionInfo>('/auth/login', credentials, { signal });
    return data;
  },

  async createGuestSession(signal?: AbortSignal): Promise<SessionInfo> {
    const { data } = await api.post<SessionInfo>('/auth/guest-session', undefined, { signal });
    return data;
  },

  async getSessionContext(signal?: AbortSignal): Promise<SessionContext> {
    const { data } = await api.get<SessionContext>('/auth/session', { signal });
    return data;
  },

  async logout(signal?: AbortSignal): Promise<void> {
    await api.post('/auth/logout', undefined, { signal, timeout: 15_000 });
  },
};
