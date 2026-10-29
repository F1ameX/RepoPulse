import { Link, useParams } from 'react-router-dom';
import { Message } from '@/components';
import { AnalysisAccess } from '@/components/Analysis/AnalysisAccess';
import { AnalysisLayout } from '@/components/Analysis/AnalysisLayout';
import { AnalysisResult } from '@/components/Analysis/AnalysisResult';

export default function ReportPage() {
  const { id } = useParams();
  const valid = id && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id);
  return (
    <AnalysisLayout>
      {valid ? (
        <AnalysisAccess>
          <AnalysisResult key={id} reportId={id} />
        </AnalysisAccess>
      ) : (
        <Message variant="error" actions={<Link to="/">Новый анализ</Link>}>
          Отчёт недоступен
        </Message>
      )}
    </AnalysisLayout>
  );
}
