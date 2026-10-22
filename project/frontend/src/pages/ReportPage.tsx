import { useParams } from 'react-router-dom';

export default function ReportPage() {
  const { id } = useParams<{ id: string }>();
  return <div>Отчёт по анализу {id}</div>;
}
