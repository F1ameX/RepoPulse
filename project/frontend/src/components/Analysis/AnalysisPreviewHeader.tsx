import type { AnalysisStatus } from '@/types';
import { ReportHeader } from './ReportHeader';

export function AnalysisPreviewHeader({ analysis }: { analysis: AnalysisStatus }) {
  // Only the status fields confirmed by the server are available in the C2 view.
  return <ReportHeader report={analysis} label="Шапка анализа С2" />;
}
