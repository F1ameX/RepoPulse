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
}: {
  evidence: Evidence[];
  limited: boolean;
  total: number | null;
  reason: Reason | null;
  limit?: number;
  secret?: boolean;
}) {
  if (!evidence || evidence.length === 0) {
    return (
      <div className={styles.evidenceBlock}>
        <strong>Основания:</strong>{' '}
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

  return (
    <div className={styles.evidenceBlock}>
      <strong>Основания:</strong>
      <ul>
        {visible.map((ev) => {
          const isSecret = secret || !!ev.secret_match_type;
          return (
            <li key={ev.id}>
              <span>
                {isSecret
                  ? ev.secret_match_type || 'Тип совпадения не определён'
                  : ev.description}
              </span>
              {!isSecret && <span>{` (источник: ${ev.source})`}</span>}
              {ev.path && <span>{` [${ev.path}]`}</span>}
              {ev.revision && <span>{` rev: ${ev.revision}`}</span>}
              {!isSecret && ev.run_id && (
                <span>{` run: ${ev.run_id}${ev.attempt !== null ? `/${ev.attempt}` : ''}`}</span>
              )}
              {!isSecret && ev.dependency && (
                <span>
                  {` ${ev.dependency.ecosystem}/${ev.dependency.package}@${ev.dependency.version}`}
                </span>
              )}
              {!isSecret && ev.vulnerability_id && <span>{` vuln: ${ev.vulnerability_id}`}</span>}
              {isSecret ? null : isSafeUrl(ev.url) ? (
                <>
                  {' '}
                  <a href={ev.url} target="_blank" rel="noopener noreferrer">
                    [ссылка]
                  </a>
                </>
              ) : (
                <span>
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
          {limited && '; список ограничен профилем'}.{total !== null && ` Всего: ${total}.`}
        </p>
      )}
    </div>
  );
}
