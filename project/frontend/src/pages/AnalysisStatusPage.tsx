import { Link, useNavigate, useParams } from 'react-router-dom';
import { Button, FeedbackMessage, Message } from '@/components';
import { statusFeedback, submissionFeedback } from '@/components/FeedbackMessage/mappers';
import { AnalysisAccess } from '@/components/Analysis/AnalysisAccess';
import { AnalysisLayout, AnalysisTitle } from '@/components/Analysis/AnalysisLayout';
import { AnalysisPreviewHeader } from '@/components/Analysis/AnalysisPreviewHeader';
import { AnalysisResult } from '@/components/Analysis/AnalysisResult';
import { useAnalysisStatus } from '@/hooks/useAnalysisStatus';
import { useAnalysisSubmission } from '@/hooks/useAnalysisSubmission';
import { analysisFailure, STAGE_LABELS, STATUS_LABELS } from '@/lib/analysisView';
import spinner from '@/assets/analysis-spinner.svg';
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
      <>
        <FeedbackMessage view={statusFeedback(state, { onRefresh: () => void refresh() })} />
        <Link to="/" className={styles.newAnalysis}>
          Новый анализ
        </Link>
      </>
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
  const metadataPreview =
    failed &&
    confirmed.reasonCode === 'ANALYSIS_FAILED' &&
    import.meta.env.VITE_C2_METADATA_PREVIEW !== 'false';
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
      <section
        className={metadataPreview ? styles.previewSection : styles.statusSection}
        aria-label="Состояние анализа"
      >
        {!metadataPreview && (
          <div className={styles.statusRow}>
            {!failed && (
              <span className={styles.spinner} aria-hidden="true">
                <img src={spinner} alt="" width="67" height="35" />
              </span>
            )}
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
        )}
        {failed ? (
          <>
            {!metadataPreview && (
              <div className={styles.feedback}>
                <FeedbackMessage
                  view={{ variant: 'error', message: analysisFailure(confirmed.reasonCode) }}
                />
              </div>
            )}
            {metadataPreview && <AnalysisPreviewHeader analysis={confirmed} />}
            <div className={styles.statusActions}>
              <Button variant="secondary" disabled={submission.isPending} onClick={retry}>
                Повторить анализ
              </Button>
              <Button
                className={styles.primaryAction}
                variant="primary"
                disabled={submission.isPending}
                onClick={changeUrl}
              >
                Новый анализ
              </Button>
            </div>
            <div className={styles.feedback}>
              <FeedbackMessage
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
            </div>
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
              <div className={styles.feedback}>
                <FeedbackMessage
                  view={(() => {
                    const view = statusFeedback(state, { onRefresh: () => void refresh() });
                    return view ? { ...view, actions: undefined } : null;
                  })()}
                />
              </div>
            )}
          </>
        )}
      </section>
    </>
  );
}
export default function AnalysisStatusPage() {
  const { id } = useParams();
  return (
    <AnalysisLayout>
      {!validId(id) ? (
        <Message
          variant="error"
          actions={
            <Link to="/" className={styles.newAnalysis}>
              Новый анализ
            </Link>
          }
        >
          Анализ недоступен
        </Message>
      ) : (
        <AnalysisAccess>
          <AnalysisLifecycle key={id} id={id!} />
        </AnalysisAccess>
      )}
    </AnalysisLayout>
  );
}
