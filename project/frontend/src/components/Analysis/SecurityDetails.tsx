import type { Dependency, MetricId, SourceState } from '@/types';
import { STATUS_LABELS } from '@/lib/metricFormatting';
import styles from './Analysis.module.css';

function sourceSampleText(source: SourceState): string {
  const { collected, total } = source.sample;
  if (collected === null && total === null) return 'Размер выборки неизвестен';
  if (collected === null) return `Всего: ${total}; полученное количество неизвестно`;
  if (total === null) return `Получено: ${collected}; полный объём неизвестен`;
  return `Получено: ${collected} из ${total}`;
}

const VERSION_SOURCE_LABELS: Record<string, string> = {
  npm_lock_v2: 'npm lock v2',
  npm_lock_v3: 'npm lock v3',
  maven_declaration: 'декларация Maven',
  maven_dependency_management: 'dependencyManagement Maven',
  go_require_declaration: 'декларация require в go.mod',
};

function isDeclaredVersion(versionSource: string): boolean {
  return ['maven_declaration', 'maven_dependency_management', 'go_require_declaration'].includes(
    versionSource,
  );
}

function dependencyOrigins(dependency: Dependency) {
  const origins = [
    { manifest_path: dependency.manifest_path, version_source: dependency.version_source },
    ...(Array.isArray(dependency.origins) ? dependency.origins : []),
  ];
  const seen = new Set<string>();
  return origins.filter((origin) => {
    if (
      !origin ||
      typeof origin.manifest_path !== 'string' ||
      typeof origin.version_source !== 'string'
    ) {
      return false;
    }
    const key = `${origin.manifest_path}\u0000${origin.version_source}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function SecurityDetails({
  metricId,
  sources,
  dependencies,
}: {
  metricId: MetricId;
  sources?: SourceState[];
  dependencies?: Dependency[];
}) {
  const osvSource = sources?.find((source) => source.source === 'osv' && source.scope === 'osv');
  const linkedSources = (sources ?? []).filter(
    (source) =>
      (metricId !== 'SEC_VULN_RISK' || source !== osvSource) &&
      (source.metric_ids?.includes(metricId) ||
        (metricId === 'SEC_SECRETS' &&
          source.source === 'git' &&
          source.scope === 'secret_history')),
  );
  return (
    <>
      {linkedSources.map((source) => (
        <div className={styles.metricSource} key={`${source.source}/${source.scope}`}>
          <div>
            <strong>Источник:</strong> {source.source} · {source.scope}:{' '}
            {STATUS_LABELS[source.status]}
          </div>
          <div className={styles.hint}>{sourceSampleText(source)}</div>
          {source.reason && (
            <div className={styles.naReason}>
              {source.reason.message}. {source.reason.cannot_conclude}
            </div>
          )}
        </div>
      ))}
      {metricId === 'SEC_VULN_RISK' && (
        <section className={styles.securityCoverage} aria-label="Покрытие экосистем OSV">
          <h4>Зависимости, переданные на проверку OSV</h4>
          {(['npm', 'Maven', 'Go'] as const).map((ecosystem) => {
            const scope = `dependencies_${ecosystem.toLowerCase()}`;
            const source = sources?.find((item) => item.source === 'git' && item.scope === scope);
            const items = (dependencies ?? []).filter((item) => item.ecosystem === ecosystem);
            return (
              <div className={styles.securityEcosystem} key={ecosystem}>
                <div className={styles.sourceSummary}>
                  <strong>{ecosystem}</strong>
                  {source && <span className={styles.hint}>{STATUS_LABELS[source.status]}</span>}
                </div>
                {source ? (
                  <>
                    <span className={styles.hint}>
                      Источник {source.source} · {source.scope}
                    </span>
                    <span className={styles.hint}>{sourceSampleText(source)}</span>
                    {source.reason && (
                      <span className={styles.naReason}>
                        {source.reason.message}. {source.reason.cannot_conclude}
                      </span>
                    )}
                  </>
                ) : (
                  <span className={styles.hint}>
                    Состояние извлечения зависимостей не передано; отсутствие записей не
                    подтверждает отсутствие зависимостей.
                  </span>
                )}
                {items.length > 0 ? (
                  <details>
                    <summary
                      className={styles.evidenceSummary}
                      aria-label={`Зависимости ${ecosystem} (${items.length})`}
                    >
                      Зависимости ({items.length})
                    </summary>
                    <ul className={styles.dependencyList}>
                      {items.map((item) => (
                        <li key={`${item.ecosystem}/${item.package}/${item.version}`}>
                          <strong>
                            {item.package}@{item.version}
                          </strong>
                          {dependencyOrigins(item).map((origin) => (
                            <div
                              className={styles.dependencyOrigin}
                              key={`${origin.manifest_path}/${origin.version_source}`}
                            >
                              <span>
                                Манифест: {origin.manifest_path} · источник версии:{' '}
                                {VERSION_SOURCE_LABELS[origin.version_source] ??
                                  origin.version_source}
                              </span>
                              {isDeclaredVersion(origin.version_source) && (
                                <div className={styles.hint}>
                                  Версия объявлена в манифесте; фактический граф сборки не
                                  подтверждён.
                                </div>
                              )}
                            </div>
                          ))}
                        </li>
                      ))}
                    </ul>
                  </details>
                ) : (
                  <span className={styles.hint}>
                    Нет сохранённых зависимостей этой экосистемы. Проверьте состояние и причину
                    выше.
                  </span>
                )}
              </div>
            );
          })}
          <div className={styles.securityEcosystem}>
            <div className={styles.sourceSummary}>
              <strong>Запросы OSV</strong>
              {osvSource && <span className={styles.hint}>{STATUS_LABELS[osvSource.status]}</span>}
            </div>
            {osvSource ? (
              <>
                <span className={styles.hint}>
                  Источник {osvSource.source} · {osvSource.scope}
                </span>
                <span className={styles.hint}>{sourceSampleText(osvSource)}</span>
                {osvSource.reason && (
                  <span className={styles.naReason}>
                    {osvSource.reason.message}. {osvSource.reason.cannot_conclude}
                  </span>
                )}
              </>
            ) : (
              <span className={styles.hint}>
                Состояние источника OSV не передано; отсутствие уязвимостей не подтверждено.
              </span>
            )}
          </div>
        </section>
      )}
    </>
  );
}
