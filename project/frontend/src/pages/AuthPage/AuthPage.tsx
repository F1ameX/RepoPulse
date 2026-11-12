import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Button, Input, Message } from '@/components';
import { GuestAction } from '@/components/Auth/GuestAction';
import { useAuth } from '@/components/Auth/AuthContext';
import { PageLayout } from '@/components/Page/PageLayout';
import pageStyles from '@/components/Page/Page.module.css';
import { validateCredentials } from '@/lib/credentials';
import { COOKIE_REQUIRED, notifySessionChanged, reportReturnTo } from '@/lib/authNavigation';
import { ApiError } from '@/services/api';
import { authService } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import styles from './AuthPage.module.css';

const LOGIN_KEY = 'repopulse.auth-login';

export default function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const register = mode === 'register';
  const location = useLocation();
  const navigate = useNavigate();
  const auth = useAuth();
  const returnTo = reportReturnTo(location.state?.returnTo);
  const [login, setLogin] = useState(() => {
    if (typeof location.state?.login === 'string') return location.state.login;
    try {
      return sessionStorage.getItem(LOGIN_KEY) ?? '';
    } catch {
      return '';
    }
  });
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const passwordId = useId();
  const [errors, setErrors] = useState<{ login?: string; password?: string }>({});
  const [feedback, setFeedback] = useState('');
  const [uncertain, setUncertain] = useState(false);
  const [pending, setPending] = useState(false);
  const active = useRef<AbortController | null>(null);
  const loginInput = useRef<HTMLInputElement>(null);
  const passwordInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const leave = () => {
      active.current?.abort();
      active.current = null;
      if (passwordInput.current) passwordInput.current.value = '';
      setPassword('');
      setShowPassword(false);
      setPending(false);
    };
    window.addEventListener('pagehide', leave);
    return () => {
      active.current?.abort();
      window.removeEventListener('pagehide', leave);
    };
  }, []);

  const rememberLogin = (value: string) => {
    setLogin(value);
    try {
      sessionStorage.setItem(LOGIN_KEY, value);
    } catch {
      return;
    }
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (active.current) return;
    const data = new FormData(event.currentTarget);
    const { credentials, errors: nextErrors } = validateCredentials(
      String(data.get('login') ?? ''),
      String(data.get('password') ?? ''),
    );
    setErrors(register ? nextErrors : {});
    setFeedback('');
    setUncertain(false);
    if (nextErrors.login || nextErrors.password) {
      if (!register) setFeedback('Проверьте логин и пароль');
      (nextErrors.login ? loginInput : passwordInput).current?.focus();
      return;
    }

    const request = new AbortController();
    active.current = request;
    const signal = AbortSignal.any([request.signal, AbortSignal.timeout(15_000)]);
    setPending(true);
    setShowPassword(false);
    rememberLogin(credentials.login);
    try {
      if (register) {
        const result = await authService.register(credentials, signal);
        signal.throwIfAborted();
        setPassword('');
        rememberLogin(result.login);
        navigate('/login', {
          replace: true,
          state: { login: result.login, registered: true, returnTo },
        });
        return;
      }

      const session = await authService.login(credentials, signal);
      signal.throwIfAborted();
      setPassword('');
      const { account } = await authService.getSessionContext(signal);
      signal.throwIfAborted();
      if (
        !account ||
        account.login !== session.login ||
        account.expires_at !== session.expires_at
      ) {
        setFeedback(COOKIE_REQUIRED);
        return;
      }
      notifySessionChanged();
      let destination = '/history';
      if (returnTo) {
        const reportId = returnTo.slice('/report/'.length);
        const report = await repositoryService.getReport(reportId, signal).catch(() => null);
        if (report?.report_id === reportId) destination = returnTo;
      }
      if (!request.signal.aborted) navigate(destination, { replace: true });
    } catch (error) {
      if (request.signal.aborted) return;
      const code = error instanceof ApiError ? error.code : '';
      if (code === 'LOGIN_TAKEN' && register) {
        setErrors({ login: 'Этот логин уже занят' });
      } else if (code === 'INVALID_CREDENTIALS' && !register) {
        setFeedback('Неверный логин или пароль');
      } else if (register && (signal.aborted || code === 'NETWORK_ERROR')) {
        setUncertain(true);
        setFeedback('Не удалось подтвердить регистрацию. Если логин уже создан, попробуйте войти');
      } else {
        setFeedback(register ? 'Не удалось завершить регистрацию' : 'Не удалось завершить вход');
      }
    } finally {
      if (active.current === request) {
        active.current = null;
        if (passwordInput.current) passwordInput.current.value = '';
        setPassword('');
        setPending(false);
      }
    }
  };

  return (
    <PageLayout>
      <h1 className={pageStyles.title}>{register ? 'Регистрация' : 'Вход'}</h1>
      {!register && location.state?.registered && (
        <Message variant="success" className={styles.feedback}>
          Аккаунт создан. Войдите в систему
        </Message>
      )}
      {!register && (location.state?.expired || auth?.expired) && (
        <Message variant="warning" className={styles.feedback}>
          Сессия истекла. Войдите снова
        </Message>
      )}
      <form
        className={styles.form}
        onSubmit={(event) => void submit(event)}
        noValidate
        aria-busy={pending}
      >
        <Input
          ref={loginInput}
          label="Логин"
          name="login"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          required
          disabled={pending}
          value={login}
          error={errors.login}
          onChange={(event) => {
            rememberLogin(event.target.value);
            setErrors({ ...errors, login: undefined });
          }}
        />
        <Input
          ref={passwordInput}
          id={passwordId}
          label="Пароль"
          name="password"
          type={showPassword ? 'text' : 'password'}
          autoComplete={register ? 'new-password' : 'current-password'}
          autoCapitalize="none"
          spellCheck={false}
          action={
            <button
              type="button"
              className={styles.passwordToggle}
              aria-controls={passwordId}
              aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
              disabled={pending}
              onClick={() => setShowPassword((visible) => !visible)}
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.7"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                {showPassword ? (
                  <>
                    <path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7-10-7-10-7Z" />
                    <circle cx="12" cy="12" r="3" />
                  </>
                ) : (
                  <path d="m3 3 18 18M10.6 10.6a2 2 0 0 0 2.8 2.8M9.9 5.2A9.7 9.7 0 0 1 12 5c6 0 10 7 10 7a18 18 0 0 1-3 3.6M6.2 6.2C3.6 8.3 2 12 2 12s4 7 10 7a9 9 0 0 0 4.2-1.1" />
                )}
              </svg>
            </button>
          }
          required
          disabled={pending}
          value={password}
          error={errors.password}
          onChange={(event) => {
            setPassword(event.target.value);
            setErrors({ ...errors, password: undefined });
          }}
        />
        {feedback && <Message variant={uncertain ? 'warning' : 'error'}>{feedback}</Message>}
        <div className={styles.actions}>
          <Button type="submit" isLoading={pending}>
            {pending
              ? register
                ? 'Создаём аккаунт'
                : 'Выполняется вход'
              : register
                ? 'Зарегистрироваться'
                : 'Войти'}
          </Button>
        </div>
      </form>
      <div className={styles.feedback}>
        <GuestAction disabled={pending} />
      </div>
    </PageLayout>
  );
}
