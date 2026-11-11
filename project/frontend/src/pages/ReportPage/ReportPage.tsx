import { useParams } from 'react-router-dom';
import ErrorPage from '../ErrorPages/ErrorPage';
import { AnalysisAccess } from '@/components/Analysis/AnalysisAccess';
import { PageLayout } from '@/components/Page/PageLayout';
import { AnalysisResult } from '@/components/Analysis/AnalysisResult';

export default function ReportPage() {
  const { id } = useParams();
  const valid = id && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id);
  if (!valid) return <ErrorPage title="Отчёт недоступен" action="Новый анализ" />;
  return (
    <PageLayout>
      <AnalysisAccess>
        <AnalysisResult key={id} reportId={id} />
      </AnalysisAccess>
    </PageLayout>
  );
}
