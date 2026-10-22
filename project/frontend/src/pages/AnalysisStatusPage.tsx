import { useParams } from 'react-router-dom';

export default function AnalysisStatusPage() {
  const { id } = useParams<{ id: string }>();
  return <div>Анализ {id} выполняется…</div>;
}
