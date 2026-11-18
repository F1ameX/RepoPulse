import { useNavigate, useParams } from 'react-router-dom';
import { Button, FeedbackMessage } from '@/components';
import ErrorPage from '../ErrorPages/ErrorPage';
import { statusFeedback, submissionFeedback } from '@/components/FeedbackMessage/mappers';
import { AnalysisAccess } from '@/components/Analysis/AnalysisAccess';
import { GuestAction } from '@/components/Auth/GuestAction';
import { ButtonLink } from '@/components/Button/ButtonLink';
import { AnalysisTitle } from '@/components/Analysis/AnalysisTitle';
import { PageLayout } from '@/components/Page/PageLayout';
import pageStyles from '@/components/Page/Page.module.css';
import { AnalysisResult } from '@/components/Analysis/AnalysisResult';
import { useAnalysisStatus } from '@/hooks/useAnalysisStatus';
import { useAnalysisSubmission } from '@/hooks/useAnalysisSubmission';
import { analysisFailure, isSessionError, STAGE_LABELS, STATUS_LABELS } from '@/lib/analysisView';
import styles from '@/components/Analysis/Analysis.module.css';

const validId = (value: string | undefined) =>
  value !== undefined &&
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);

function AnalysisLifecycle({ id }: { id: string }) {
  const navigate = useNavigate();
  const { state, refresh, isPending } = useAnalysisStatus(id);
  const submission = useAnalysisSubmission({
    onAccepted: (response) => navigate(`/analysis/${response.analysis_id}`),
  });
  if (state.kind === 'hidden')
    return (
      <div className={pageStyles.recovery}>
        <FeedbackMessage view={statusFeedback(state, { onRefresh: () => void refresh() })} />
        {isSessionError(state.code) ? (
          <>
            <ButtonLink
              to="/login"
              variant="secondary"
              state={{ expired: state.code === 'SESSION_EXPIRED' }}
            >
              Войти
            </ButtonLink>
            <GuestAction />
          </>
        ) : (
          <ButtonLink to="/">Новый анализ</ButtonLink>
        )}
      </div>
    );
  const confirmed = 'lastConfirmed' in state ? state.lastConfirmed : null;
  if (confirmed?.status === 'completed' && confirmed.reportId) {
    return (
      <AnalysisResult
        key={confirmed.reportId}
        reportId={confirmed.reportId}
        analysisId={id}
        repository={confirmed.repository}
      />
    );
  }
  const failed = confirmed?.status === 'failed';
  const stage = confirmed?.stage ? STAGE_LABELS[confirmed.stage] : undefined;
  const changeUrl = () =>
    navigate('/', {
      state: { repositoryUrl: confirmed?.repository.canonical_url ?? '', previousAnalysisId: id },
    });
  const retry = () => {
    if (confirmed) submission.retryNew(confirmed.repository.canonical_url);
  };
  return (
    <>
      <AnalysisTitle repository={confirmed?.repository} />
      {failed && <p className={styles.notice}>Анализ: {id}</p>}
      <section className={styles.statusSection} aria-label="Состояние анализа">
        <div className={styles.statusRow}>
          {!failed && <span className={styles.spinner} aria-hidden="true" />}
          <div aria-live="polite">
            <p className={styles.statusText}>
              {confirmed && 'Состояние анализа: '}
              <strong>
                {confirmed ? STATUS_LABELS[confirmed.status] : 'Состояние пока неизвестно'}
              </strong>
            </p>
            {stage && !failed && <p className={styles.stage}>{stage}</p>}
          </div>
        </div>
        {failed ? (
          <>
            <FeedbackMessage
              className={styles.feedback}
              view={{
                variant: 'error',
                message: analysisFailure(confirmed.reasonCode, confirmed.error?.message),
              }}
            />
            <div className={styles.statusActions}>
              <Button variant="secondary" disabled={submission.isPending} onClick={retry}>
                Повторить анализ
              </Button>
              <Button variant="secondary" disabled={submission.isPending} onClick={changeUrl}>
                Изменить URL
              </Button>
            </div>
            <FeedbackMessage
              className={styles.feedback}
              view={
                submission.state.kind === 'submitting'
                  ? null
                  : submissionFeedback(submission.state, confirmed.repository.canonical_url, {
                      retrySame: submission.retrySame,
                      retryNew: retry,
                      onChangeUrl: changeUrl,
                      onOpen: (analysisId) => navigate(`/analysis/${analysisId}`),
                    })
              }
            />
          </>
        ) : (
          <>
            <div className={styles.statusActions}>
              <Button variant="secondary" disabled={isPending} onClick={() => void refresh()}>
                {state.kind === 'error' || state.kind === 'offline'
                  ? 'Повторить проверку статуса'
                  : 'Проверить статус'}
              </Button>
            </div>
            {(state.kind === 'error' || state.kind === 'offline') && (
              <FeedbackMessage
                className={styles.feedback}
                view={(() => {
                  const view = statusFeedback(state, { onRefresh: () => void refresh() });
                  return view ? { ...view, actions: undefined } : null;
                })()}
              />
            )}
          </>
        )}
      </section>
    </>
  );
}
export default function AnalysisStatusPage() {
  const { id } = useParams();
  if (!validId(id)) return <ErrorPage title="Анализ недоступен" action="Новый анализ" />;
  return (
    <PageLayout>
      <AnalysisAccess>
        <AnalysisLifecycle key={id} id={id!} />
      </AnalysisAccess>
    </PageLayout>
  );
}
