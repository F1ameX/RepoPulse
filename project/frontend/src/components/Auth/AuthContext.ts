import { createContext, useContext } from 'react';
import type { SessionInfo } from '@/services/authService';

export const AuthContext = createContext<{
  account: SessionInfo | null;
  checking: boolean;
  expired: boolean;
  revision: number;
  logoutState: 'idle' | 'pending' | 'error';
  logout: () => Promise<boolean>;
} | null>(null);

export function useAuth() {
  return useContext(AuthContext);
}
