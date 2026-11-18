import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Message } from '@/components';
import { useAuth } from '@/components/Auth/AuthContext';
import { GuestAction } from '@/components/Auth/GuestAction';
import { ButtonLink } from '@/components/Button/ButtonLink';
import { PageLayout } from '@/components/Page/PageLayout';
import pageStyles from '@/components/Page/Page.module.css';
import { formatUtc, isSessionError } from '@/lib/analysisView';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { HistoryPage as HistoryResult } from '@/types';
import styles from './HistoryPage.module.css';

function HistoryList() {
  const [page, setPage] = useState<HistoryResult | null>(null);
  const [cursor, setCursor] = useState<string | undefined>();
  const [attempt, setAttempt] = useState(0);
  const [errorCode, setErrorCode] = useState('');
  useEffect(() => {
    const request = new AbortController();
    void repositoryService.listReports(cursor, request.signal).then(
      (result) => {
        if (!request.signal.aborted) setPage(result);
      },
      (failure: unknown) => {
        if (request.signal.aborted) return;
        setPage(null);
        setErrorCode(failure instanceof ApiError ? failure.code : 'NETWORK_ERROR');
      },
    );
    return () => request.abort();
  }, [cursor, attempt]);

  if (isSessionError(errorCode)) return <HistorySignIn expired />;
  if (errorCode)
    return (
      <>
        <Message variant="warning">
          {errorCode === 'NOT_IMPLEMENTED'
            ? 'История отчётов пока недоступна'
            : 'Не удалось загрузить историю'}
        </Message>
        <div className={styles.actions}>
          <Button
            variant="secondary"
            onClick={() => {
              setErrorCode('');
              setAttempt(attempt + 1);
            }}
          >
            Повторить загрузку
          </Button>
        </div>
      </>
    );
  if (!page) return <Message variant="info">Загружаем историю</Message>;
  return (
    <>
      {page.items.length ? (
        <ul>
          {page.items.map((report) => (
            <li key={report.report_id}>
              <Link to={`/report/${report.report_id}`}>
                {report.repository.owner}/{report.repository.name}
              </Link>
              {' · '}
              {formatUtc(report.completed_at)}
            </li>
          ))}
        </ul>
      ) : (
        <p>Сохранённых отчётов пока нет</p>
      )}
      {page.next_cursor && (
        <Button
          variant="secondary"
          onClick={() => {
            setPage(null);
            setCursor(page.next_cursor ?? undefined);
          }}
        >
          Следующая страница
        </Button>
      )}
    </>
  );
}

function HistorySignIn({ expired }: { expired?: boolean }) {
  return (
    <>
      <Message variant="warning">
        {expired ? 'Сессия истекла. Войдите снова' : 'Войдите, чтобы открыть историю отчётов'}
      </Message>
      <div className={styles.actions}>
        <ButtonLink to="/login" variant="secondary" state={{ expired }}>
          Войти
        </ButtonLink>
        <ButtonLink to="/register" variant="secondary">
          Зарегистрироваться
        </ButtonLink>
        <GuestAction />
      </div>
    </>
  );
}

export default function HistoryPage() {
  const auth = useAuth();
  const login = auth?.account?.login;
  return (
    <PageLayout>
      <h1 className={pageStyles.title}>{login || 'История отчётов'}</h1>
      {login && <h2 className={styles.subtitle}>История отчётов</h2>}
      <div className={styles.content}>
        {auth?.checking ? (
          <Message variant="info">Проверяем сессию</Message>
        ) : auth?.account ? (
          <HistoryList key={JSON.stringify(auth.account)} />
        ) : (
          <HistorySignIn expired={auth?.expired} />
        )}
      </div>
    </PageLayout>
  );
}
