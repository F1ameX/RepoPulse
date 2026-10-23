import api from './api';

export const authService = {
  async createGuestSession(): Promise<void> {
    await api.post('/auth/guest-session');
  },

  async getSessionContext(): Promise<unknown> {
    const { data } = await api.get('/auth/session');
    return data;
  },

  async logout(): Promise<void> {
    await api.post('/auth/logout');
  },
};