import { useLocation, useNavigate } from 'react-router-dom';
import { FeedbackMessage, Button, Input, Message } from '@/components';
import { submissionFeedback } from '@/components/FeedbackMessage/mappers';
import { useAnalysisSubmission } from '@/hooks/useAnalysisSubmission';
import { useRepositoryValidation } from '@/hooks/useRepositoryValidation';
import styles from './RepositoryInputPage.module.css';

export default function RepositoryInputPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const initialUrl =
    typeof location.state?.repositoryUrl === 'string' ? location.state.repositoryUrl : '';
  const submission = useAnalysisSubmission({
    onAccepted: (response) => navigate(`/analysis/${response.analysis_id}`),
  });
  const onValidated = (url: string) => {
    if (submission.hasUnconfirmed) {
      if (url === submission.originalUrl) {
        submission.retrySame();
        return;
      }
      if (!window.confirm('Прежний запрос мог быть принят. Отправить новый анализ с другим URL?'))
        return;
    }
    submission.submit(url);
  };
  const launchFeedback = (
    <FeedbackMessage
      view={submissionFeedback(submission.state, undefined, {
        retrySame: submission.retrySame,
        retryNew: () => submission.retryNew(submission.originalUrl ?? initialUrl),
        onChangeUrl: () => {
          if (
            submission.hasUnconfirmed &&
            !window.confirm('Прежний запрос мог быть принят. Изменить URL для нового анализа?')
          )
            return;
          const url = submission.originalUrl ?? initialUrl;
          submission.reset();
          setUrl(url);
          navigate('/', { state: { repositoryUrl: url }, replace: true });
        },
        onOpen: (id) => navigate(`/analysis/${id}`),
      })}
    />
  );
  const {
    url,
    state,
    isPending: validationPending,
    inputRef,
    setUrl,
    submit,
    retry,
    fix,
  } = useRepositoryValidation({ initialUrl, onValidated });
  const isPending = validationPending || submission.isPending;
  const actions = (
    <>
      <Button variant="secondary" size="sm" onClick={retry}>
        Повторить
      </Button>
      <Button variant="secondary" size="sm" onClick={fix}>
        Изменить URL
      </Button>
    </>
  );
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <span className={styles.logo}>RepoPulse</span>
      </header>
      <main className={styles.content}>
        <section className={styles.hero} aria-labelledby="project-title">
          <h1 id="project-title" className={styles.title}>
            Веб-сервис для автоматической оценки технического здоровья GitHub-репозитория
          </h1>
          <p className={styles.description}>
            RepoPulse анализирует историю, документацию, тесты, зависимости, CI/CD и code review — и
            выдаёт Repo Health Score от 0 до 100 с объяснением и рекомендациями.
          </p>
          <form
            className={styles.form}
            onSubmit={(event) => {
              event.preventDefault();
              submit();
            }}
            noValidate
            aria-busy={isPending}
          >
            <label className={styles.label} htmlFor="repository-url">
              URL репозитория
            </label>
            <div className={styles.controls}>
              <Input
                id="repository-url"
                ref={inputRef}
                className={styles.input}
                aria-invalid={state.kind === 'field-error'}
                aria-describedby={
                  state.kind === 'field-error'
                    ? 'repository-hint repository-error'
                    : 'repository-hint'
                }
                type="text"
                inputMode="url"
                autoCapitalize="none"
                spellCheck={false}
                placeholder="https://github.com/username/repo"
                value={url}
                onChange={(event) => setUrl(event.target.value)}
                onPaste={(event) => {
                  event.preventDefault();
                  const input = event.currentTarget;
                  setUrl(
                    url.slice(0, input.selectionStart ?? url.length) +
                      event.clipboardData.getData('text') +
                      url.slice(input.selectionEnd ?? url.length),
                  );
                }}
                size="lg"
                required
                disabled={isPending}
              />
              <Button className={styles.submit} type="submit" size="lg" disabled={isPending}>
                Анализировать
              </Button>
            </div>
            <p className={styles.hint} id="repository-hint">
              Поддерживаются публичные репозитории GitHub
            </p>
            <div className={styles.feedback}>
              {launchFeedback}
              {state.kind === 'validated' && (
                <>
                  {state.movedTo && (
                    <Message variant="info">Репозиторий перемещён: {state.movedTo}</Message>
                  )}
                  <Message variant="success">Репозиторий доступен. URL прошёл проверку.</Message>
                </>
              )}
              {state.kind === 'unavailable' && (
                <Message variant="error" actions={actions}>
                  Публичный репозиторий недоступен
                </Message>
              )}
              {state.kind === 'temporary-error' && (
                <Message variant="warning" actions={actions}>
                  {state.message}
                </Message>
              )}
              {state.kind === 'session-error' && (
                <Message variant="error" actions={actions}>
                  {state.message}
                </Message>
              )}
              {state.kind === 'field-error' && (
                <div id="repository-error">
                  <Message variant="error">{state.message}</Message>
                </div>
              )}
            </div>
          </form>
        </section>
      </main>
    </div>
  );
}
