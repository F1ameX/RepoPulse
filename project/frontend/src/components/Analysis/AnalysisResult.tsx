import { Link } from 'react-router-dom';
import { Button, FeedbackMessage, Message } from '@/components';
import { reportFeedback } from '@/components/FeedbackMessage/mappers';
import { useReport } from '@/hooks/useReport';
import type { Repository } from '@/types';
import { AnalysisTitle } from './AnalysisLayout';
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
        <Link to="/">Новый анализ</Link>
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
      <div className={styles.feedback}>
        <FeedbackMessage view={reportFeedback(state, { onReload: () => void reload() })} />
      </div>
      {report && <ReportHeader report={report} />}
    </>
  );
}
