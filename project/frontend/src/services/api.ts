import axios, { AxiosError } from 'axios';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '/api/v1',
  withCredentials: true,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  },
});

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status?: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string };
  code?: string;
  message?: string;
}

api.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ErrorBody>) => {
    if (!error.response) {
      return Promise.reject(
        new ApiError('NETWORK_ERROR', 'Нет соединения с сервером'),
      );
    }
    const { status, data } = error.response;
    const payload = data?.error ?? data ?? {};
    return Promise.reject(
      new ApiError(
        payload.code ?? 'INTERNAL_ERROR',
        payload.message ?? `HTTP ${status}`,
        status,
      ),
    );
  },
);

export default api;