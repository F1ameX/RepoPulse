import type { Evidence, Reason } from '@/types';
import { isSafeUrl } from '@/lib/metricFormatting';
import styles from './Analysis.module.css';

export function EvidenceList({
  evidence,
  limited,
  total,
  reason,
  limit,
}: {
  evidence: Evidence[];
  limited: boolean;
  total: number | null;
  reason: Reason | null;
  limit?: number;
}) {
  if (!evidence || evidence.length === 0) {
    return (
      <div className={styles.evidenceBlock}>
        <strong>Основания:</strong>{' '}
        <span className={styles.mono}>
          Основания недоступны
          {reason ? ` (${reason.message})` : ' (причина не указана)'}
        </span>
      </div>
    );
  }

  const visible = limit !== undefined ? evidence.slice(0, limit) : evidence;
  const truncated = visible.length < evidence.length;
  const showHint = limited || truncated;

  return (
    <div className={styles.evidenceBlock}>
      <strong>Основания:</strong>
      <ul>
        {visible.map((ev) => {
          const secret = !!ev.secret_match_type;
          return (
            <li key={ev.id}>
              <span>{ev.secret_match_type ?? ev.description}</span>
              {!secret && (
                <span className={styles.mono}>{` (источник: ${ev.source})`}</span>
              )}
              {ev.path && <span className={styles.mono}>{` [${ev.path}]`}</span>}
              {ev.revision && <span className={styles.mono}>{` rev: ${ev.revision}`}</span>}
              {!secret && ev.run_id && (
                <span className={styles.mono}>
                  {` run: ${ev.run_id}${ev.attempt !== null ? `/${ev.attempt}` : ''}`}
                </span>
              )}
              {!secret && ev.dependency && (
                <span className={styles.mono}>
                  {` ${ev.dependency.ecosystem}/${ev.dependency.package}@${ev.dependency.version}`}
                </span>
              )}
              {!secret && ev.vulnerability_id && (
                <span className={styles.mono}>{` vuln: ${ev.vulnerability_id}`}</span>
              )}
              {secret ? null : isSafeUrl(ev.url) ? (
                <>
                  {' '}
                  <a href={ev.url} target="_blank" rel="noopener noreferrer">
                    [ссылка]
                  </a>
                </>
              ) : (
                <span className={styles.mono}>
                  {' '}
                  (Ссылка на источник недоступна)
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
          {limited && '; список ограничен профилем'}.
          {total !== null && ` Всего: ${total}.`}
        </p>
      )}
    </div>
  );
}