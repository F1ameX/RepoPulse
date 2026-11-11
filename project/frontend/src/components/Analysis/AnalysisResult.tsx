import { Button, FeedbackMessage, Message } from '@/components';
import { ButtonLink } from '@/components/Button/ButtonLink';
import { reportFeedback } from '@/components/FeedbackMessage/mappers';
import { useReport } from '@/hooks/useReport';
import type { Repository } from '@/types';
import { ActivityMetrics } from './ActivityMetrics';
import { AnalysisTitle } from './AnalysisTitle';
import { ReportHeader } from './ReportHeader';
import styles from './Analysis.module.css';

export function AnalysisResult({
  reportId,
  analysisId,
  repository,
}: {
  reportId: string;
  analysisId?: string;
  repository?: Repository;
}) {
  const { state, reload } = useReport(reportId);
  if (state.kind === 'hidden')
    return (
      <>
        <FeedbackMessage view={reportFeedback(state, { onReload: () => void reload() })} />
        <ButtonLink to="/">Новый анализ</ButtonLink>
      </>
    );
  const report = 'data' in state ? state.data : null;
  const mismatch = report !== null && analysisId !== undefined && report.analysis_id !== analysisId;
  if (mismatch)
    return (
      <Message
        variant="warning"
        actions={
          <Button variant="secondary" onClick={() => void reload()}>
            Повторить загрузку
          </Button>
        }
      >
        Не удалось загрузить отчёт
      </Message>
    );
  return (
    <>
      <AnalysisTitle repository={report?.repository ?? repository} />
      {!report && analysisId && (
        <p className={styles.notice}>
          Состояние анализа: <strong>Завершён</strong>
        </p>
      )}
      <FeedbackMessage
        className={styles.feedback}
        view={reportFeedback(state, { onReload: () => void reload() })}
      />
      {report && (
        <>
          <ReportHeader report={report} repositoryInTitle />
          <ActivityMetrics report={report} />
        </>
      )}
    </>
  );
}
