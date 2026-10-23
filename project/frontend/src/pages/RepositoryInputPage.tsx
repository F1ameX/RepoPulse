import { useNavigate } from "react-router-dom";
import { Button, Input, Message } from "@/components";
import { useRepositoryValidation } from "@/hooks/useRepositoryValidation";
import styles from "./RepositoryInputPage.module.css";

export default function RepositoryInputPage() {
  const navigate = useNavigate();
  const {
    url,
    state,
    isPending,
    inputRef,
    setUrl,
    submit,
    retry,
    fix,
    useNew,
  } = useRepositoryValidation({
    onAnalysisCreated: (id) => navigate(`/analysis/${id}`),
  });

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <span className={styles.logo}>RepoPulse</span>
      </header>
      <main className={styles.content}>
        <section className={styles.hero} aria-labelledby="project-title">
          <h1 id="project-title" className={styles.title}>
            Веб-сервис для автоматической оценки технического здоровья
            GitHub-репозитория
          </h1>
          <h3 className={styles.description}>
            RepoPulse анализирует историю, документацию, тесты, зависимости,
            CI/CD и code review — и выдаёт Repo Health Score от 0 до 100 с
            объяснением и рекомендациями.
          </h3>

          <form
            className={styles.form}
            onSubmit={(event) => {
              event.preventDefault();
              submit();
            }}
            noValidate
            aria-busy={isPending}
          >
            <label className={styles.label} htmlFor="repository-url">
              URL репозитория
            </label>
            <div className={styles.controls}>
              <Input
                id="repository-url"
                ref={inputRef}
                className={styles.input}
                type="url"
                placeholder="https://github.com/username/repo"
                value={url}
                onChange={(event) => setUrl(event.target.value)}
                aria-describedby="repository-hint"
                size="lg"
                required
                disabled={isPending}
              />
              <Button
                className={styles.submit}
                type="submit"
                size="lg"
                isLoading={isPending}
              >
                Анализировать
              </Button>
            </div>
            <p className={styles.hint} id="repository-hint">
              Поддерживаются публичные репозитории GitHub
            </p>

            <div className={styles.feedback}>
              {state.kind === "pending" && (
                <Message variant="info" icon={false}>
                  Проверяем доступность репозитория…
                </Message>
              )}

              {state.kind === "validated" && (
                <Message variant="success">
                  Репозиторий доступен. URL прошёл проверку.
                </Message>
              )}

              {state.kind === "permanent-error" && (
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

              {state.kind === "temporary-error" && (
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

              {state.kind === "moved" && (
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
            </div>
          </form>
        </section>
      </main>
    </div>
  );
}
