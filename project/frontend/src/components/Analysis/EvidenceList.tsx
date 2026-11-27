import type { Evidence, Reason } from '@/types';
import { isSafeUrl } from '@/lib/metricFormatting';
import styles from './Analysis.module.css';

export function EvidenceList({
  evidence,
  limited,
  total,
  reason,
  limit,
  secret = false,
  collapsible = false,
  showHeading = true,
}: {
  evidence: Evidence[];
  limited: boolean;
  total: number | null;
  reason: Reason | null;
  limit?: number;
  secret?: boolean;
  collapsible?: boolean;
  showHeading?: boolean;
}) {
  if (!evidence || evidence.length === 0) {
    return (
      <div className={styles.evidenceBlock}>
        {!collapsible && showHeading && <strong>Основания: </strong>}
        <span>
          Основания недоступны
          {reason ? ` (${reason.message})` : ' (причина не указана)'}
        </span>
      </div>
    );
  }

  const visible = limit !== undefined ? evidence.slice(0, limit) : evidence;
  const truncated = visible.length < evidence.length;
  const showHint = limited || truncated;

  const content = (
    <>
      <ul className={styles.evidenceList}>
        {visible.map((ev) => {
          const isSecret = secret || !!ev.secret_match_type;
          return (
            <li key={ev.id}>
              <div>
                {isSecret ? ev.secret_match_type || 'Тип совпадения не определён' : ev.description}
              </div>
              <div className={styles.evidenceMeta}>
                {!isSecret && <span>{` (источник: ${ev.source})`}</span>}
                {ev.path && (
                  <span>
                    Путь: <span>{ev.path}</span>
                  </span>
                )}
                {ev.revision && (
                  <span>
                    {'Ревизия: '}
                    <span>{ev.revision}</span>
                  </span>
                )}
                {!isSecret && ev.run_id && (
                  <span>{`Запуск CI: ${ev.run_id}${ev.attempt !== null ? `/${ev.attempt}` : ''}`}</span>
                )}
                {!isSecret && ev.dependency && (
                  <span>
                    {` ${ev.dependency.ecosystem}/${ev.dependency.package}@${ev.dependency.version}`}
                  </span>
                )}
                {!isSecret && ev.vulnerability_id && (
                  <span>{`Уязвимость: ${ev.vulnerability_id}`}</span>
                )}
              </div>
              {isSecret ? null : isSafeUrl(ev.url) ? (
                <>
                  {' '}
                  <a href={ev.url} target="_blank" rel="noopener noreferrer">
                    [ссылка]
                  </a>
                </>
              ) : (
                <span className={styles.hint}>
                  Ссылка на источник недоступна
                  {ev.url && ` ${ev.url}`}
                </span>
              )}
            </li>
          );
        })}
      </ul>
      {showHint && (
        <p className={styles.hint}>
          Показано {visible.length}
          {truncated && ` из ${evidence.length}`} оснований
          {limited && '; список ограничен профилем'}.{total !== null && ` Всего: ${total}.`}
        </p>
      )}
    </>
  );

  return collapsible ? (
    <details className={styles.evidenceBlock} data-evidence>
      <summary className={styles.evidenceSummary}>Источники ({visible.length})</summary>
      {content}
    </details>
  ) : (
    <div className={styles.evidenceBlock}>
      {showHeading && <strong>Основания:</strong>}
      {content}
    </div>
  );
}
