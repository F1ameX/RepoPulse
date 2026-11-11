import type { Evidence, Reason } from '@/types';
import { isSafeUrl } from '@/lib/metricFormatting';
import styles from './Analysis.module.css';

export function EvidenceList({
  evidence,
  limited,
  total,
  reason,
}: {
  evidence: Evidence[];
  limited: boolean;
  total: number | null;
  reason: Reason | null;
}) {
  if (!evidence || evidence.length === 0) {
    return (
      <div className={styles.evidenceBlock}>
        <strong>Основания:</strong>{' '}
        <span className={styles.mono}>
          Основания недоступны
          {reason ? ` (${reason.message})` : ''}
        </span>
      </div>
    );
  }

  return (
    <div className={styles.evidenceBlock}>
      <strong>Основания:</strong>
      <ul>
        {evidence.map((ev) => (
          <li key={ev.id}>
            <span>{ev.secret_match_type ?? ev.description}</span>
            {!ev.secret_match_type && (
              <span className={styles.mono}>{` (источник: ${ev.source})`}</span>
            )}
            {ev.path && <span className={styles.mono}>{` [${ev.path}]`}</span>}
            {ev.revision && <span className={styles.mono}>{` rev: ${ev.revision}`}</span>}
            {ev.secret_match_type ? null : isSafeUrl(ev.url) ? (
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
        ))}
      </ul>
      {limited && (
        <p className={styles.hint}>
          Показано {evidence.length} оснований; список ограничен профилем.
          {total !== null && ` Всего: ${total}.`}
        </p>
      )}
    </div>
  );
}