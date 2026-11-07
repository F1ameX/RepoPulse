import { useLocation, useNavigate } from 'react-router-dom';
import { Button, Message } from '@/components';
import { reportReturnTo } from '@/lib/authNavigation';
import { ButtonLink } from '@/components/Button/ButtonLink';
import { useAuth } from './AuthContext';

export function AuthNavigation() {
  const auth = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  if (!auth) return null;
  if (auth.logoutState !== 'idle') {
    return (
      <Button
        size="sm"
        isLoading={auth.logoutState === 'pending'}
        onClick={async () => {
          if (await auth.logout()) navigate('/', { replace: true });
        }}
      >
        {auth.logoutState === 'error' ? 'Повторить выход' : 'Выходим'}
      </Button>
    );
  }
  if (auth.account) {
    return (
      <>
        <ButtonLink to="/history" variant="secondary" size="sm">
          История отчётов
        </ButtonLink>
        <ButtonLink to="/" variant="secondary" size="sm">
          Новый анализ
        </ButtonLink>
        <Button
          size="sm"
          onClick={async () => {
            if (await auth.logout()) navigate('/', { replace: true });
          }}
        >
          Выйти
        </Button>
      </>
    );
  }
  const returnTo = reportReturnTo(location.pathname) ?? reportReturnTo(location.state?.returnTo);
  return (
    <>
      <ButtonLink to="/login" variant="secondary" size="sm" state={{ returnTo }}>
        Войти
      </ButtonLink>
      <ButtonLink to="/register" size="sm" state={{ returnTo }}>
        Зарегистрироваться
      </ButtonLink>
    </>
  );
}

export function LogoutFeedback({ pending }: { pending: boolean }) {
  return (
    <Message variant={pending ? 'info' : 'warning'}>
      {pending ? 'Завершаем сессию' : 'Не удалось подтвердить выход'}
    </Message>
  );
}
