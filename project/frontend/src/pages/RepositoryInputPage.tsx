import { useNavigate } from 'react-router-dom';
import { Button, Input, Message } from '@/components';
import { useRepositoryValidation } from '@/hooks/useRepositoryValidation';

/* меняй под свой дизайн с фигмоы */

export default function RepositoryInputPage() {
  const navigate = useNavigate();
  const { url, state, isPending, inputRef, setUrl, submit, retry, fix, useNew } =
    useRepositoryValidation({
      onAnalysisCreated: (id) => navigate(`/analysis/${id}`),
    });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
      noValidate
    >
      <Input
        ref={inputRef}
        label="URL репозитория"
        type="url"
        placeholder="https://github.com/username/repo"
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        size="lg"
        required
        disabled={isPending}
      />

      {state.kind === 'pending' && (
        <Message variant="info" icon={false}>
          Проверяем доступность репозитория…
        </Message>
      )}

      {state.kind === 'validated' && (
        <Message variant="success">Репозиторий доступен. URL прошёл проверку.</Message>
      )}

      {state.kind === 'permanent-error' && (
        <Message
          variant="error"
          actions={
            <Button variant="secondary" size="sm" onClick={fix}>
              Изменить URL
            </Button>
          }
        >
          {state.message}
        </Message>
      )}

      {state.kind === 'temporary-error' && (
        <Message
          variant="warning"
          actions={
            <Button variant="secondary" size="sm" onClick={retry}>
              Повторить
            </Button>
          }
        >
          {state.message}
        </Message>
      )}

      {state.kind === 'moved' && (
        <Message
          variant="info"
          actions={
            <Button variant="secondary" size="sm" onClick={useNew}>
              Использовать новый
            </Button>
          }
        >
          Репозиторий переименован. Новый адрес: {state.newUrl}
        </Message>
      )}

      <Button type="submit" size="lg" isLoading={isPending}>
        Анализировать
      </Button>
    </form>
  );
}
