import { Link } from 'react-router-dom';

export default function ErrorPage() {
  return (
    <div role="alert">
      <p>Не удалось открыть страницу.</p>
      <Link to="/">К проверке репозитория</Link>
    </div>
  );
}
