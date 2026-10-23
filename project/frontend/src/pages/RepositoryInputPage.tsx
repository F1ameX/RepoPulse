import { useEffect, useRef, useState } from "react";
import type { SubmitEvent } from "react";
import { Button, Input, Message } from "@/components";
import { ApiError } from "@/services/api";
import { repositoryService } from "@/services/repositoryService";
import infoIcon from "@/assets/info.svg";
import {
  REPOSITORY_MESSAGES,
  readConfirmedRepository,
  repositoryErrorMessage,
  validateRepositoryUrl,
} from "./repositoryInput";
import styles from "./RepositoryInputPage.module.css";

type PageState =
  | { kind: "idle" }
  | { kind: "pending" }
  | { kind: "error"; message: string }
  | { kind: "success"; movedName?: string };

export default function RepositoryInputPage() {
  const [url, setUrl] = useState("");
  const [state, setState] = useState<PageState>({ kind: "idle" });
  const inputRef = useRef<HTMLInputElement>(null);
  const activeRequest = useRef<symbol | null>(null);
  const isPending = state.kind === "pending";

  useEffect(
    () => () => {
      activeRequest.current = null;
    },
    [],
  );

  function changeUrl(value: string) {
    if (activeRequest.current) return;
    setUrl(value);
    setState({ kind: "idle" });
  }

  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    if (activeRequest.current) return;

    const checked = validateRepositoryUrl(url);

    if (!checked.ok) {
      setState({ kind: "error", message: checked.message });
      inputRef.current?.focus();

      return;
    }

    const request = Symbol("repository-validation");
    activeRequest.current = request;

    setState({ kind: "pending" });

    try {
      const response: unknown = await repositoryService.validateRepository(
        checked.canonicalUrl,
      );

      if (activeRequest.current !== request) return;

      const confirmed = readConfirmedRepository(response, checked.canonicalUrl);

      if (!confirmed)
        throw new ApiError("INVALID_RESPONSE", "Invalid repository response");

      setUrl(confirmed.canonicalUrl);
      setState({ kind: "success", movedName: confirmed.movedName });
    } catch (error: unknown) {
      if (activeRequest.current !== request) return;

      const code = error instanceof ApiError ? error.code : "UNKNOWN_ERROR";

      setState({ kind: "error", message: repositoryErrorMessage(code) });

      inputRef.current?.focus();
    } finally {
      if (activeRequest.current === request) activeRequest.current = null;
    }
  }

  return (
    <main className={styles.page}>
      <section className={styles.hero} aria-labelledby="repository-title">
        <h1 id="repository-title" className={styles.title}>
          Веб-сервис для автоматической оценки технического здоровья
          GitHub-репозитория
        </h1>
        <p className={styles.description}>
          RepoPulse анализирует историю, документацию, тесты, зависимости, CI/CD
          и code review — и выдаёт Repo Health Score от 0 до 100 с объяснением и
          рекомендациями.
        </p>
        <form
          className={styles.form}
          onSubmit={submit}
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
              type="text"
              inputMode="url"
              autoCapitalize="none"
              spellCheck={false}
              autoComplete="url"
              placeholder="https://github.com/F1ameX/RepoPulse"
              value={url}
              readOnly={isPending}
              aria-required="true"
              aria-invalid={state.kind === "error"}
              aria-describedby={
                state.kind === "idle"
                  ? "repository-hint"
                  : "repository-hint repository-feedback"
              }
              onChange={(event) => changeUrl(event.target.value)}
              onPaste={(event) => {
                event.preventDefault();
                const input = event.currentTarget;
                changeUrl(
                  url.slice(0, input.selectionStart ?? 0) +
                    event.clipboardData.getData("text/plain") +
                    url.slice(input.selectionEnd ?? url.length),
                );
              }}
            />
            <Button
              type="submit"
              className={styles.submit}
              disabled={isPending}
            >
              анализировать
            </Button>
          </div>
          <p className={styles.hint} id="repository-hint">
            <img src={infoIcon} alt="" />
            <span>Поддерживаются публичные репозитории GitHub</span>
          </p>
          <div className={styles.feedback} id="repository-feedback">
            {state.kind === "pending" && (
              <Message className={styles.message} icon={false}>
                {REPOSITORY_MESSAGES.pending}
              </Message>
            )}
            {state.kind === "error" && (
              <Message className={styles.message} variant="error">
                {state.message}
              </Message>
            )}
            {state.kind === "success" && (
              <>
                <Message className={styles.message} variant="success">
                  {REPOSITORY_MESSAGES.available}
                </Message>
                {state.movedName && (
                  <Message className={styles.message}>
                    {REPOSITORY_MESSAGES.moved(state.movedName)}
                  </Message>
                )}
              </>
            )}
          </div>
        </form>
      </section>
    </main>
  );
}
