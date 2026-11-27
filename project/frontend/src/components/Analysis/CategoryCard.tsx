import { useId } from 'react';
import type { Category, CategoryId, Dependency, SourceState } from '@/types';
import {
  formatCoverage,
  formatSample,
  isKnownStatus,
  METRIC_LABELS,
  roundHalfUp,
  STATUS_LABELS,
} from '@/lib/metricFormatting';
import { MetricDetails } from './MetricDetails';
import styles from './Analysis.module.css';

const CATEGORY_LABELS: Partial<Record<CategoryId, string>> = {
  activity: 'Активность',
  documentation: 'Документация',
  testing: 'Тестирование',
  security: 'Безопасность',
  ci_cd: 'CI/CD',
};

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
  return [
    'maven_declaration',
    'maven_dependency_management',
    'go_require_declaration',
  ].includes(versionSource);
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

function emptyScoreLabel(category: Category): string {
  if (
    category.metrics.length > 0 &&
    category.metrics.every((metric) => metric.input.status === 'not_applicable')
  ) {
    return 'Неприменимо';
  }
  return 'Нет данных';
}

export function CategoryCard({
  category,
  sha,
  title,
  defaultOpen = false,
  reportDetails = false,
  sources,
  dependencies,
}: {
  category: Category;
  sha?: string | null;
  title?: string;
  defaultOpen?: boolean;
  reportDetails?: boolean;
  sources?: SourceState[];
  dependencies?: Dependency[];
}) {
  const label = title ?? CATEGORY_LABELS[category.id] ?? category.id;
  const explanationId = useId();
  const explanation = category.explanation?.trim();

  if (!isKnownStatus(category.status)) {
    return (
      <section className={styles.categoryCard} aria-label={`Категория: ${label}`}>
        <header className={styles.categoryHeader}>
          <h3 className={styles.categoryTitle}>{label}</h3>
        </header>
        <p className={styles.naReason} role="alert">
          Не удалось отобразить категорию: отсутствует состояние полноты данных.
        </p>
      </section>
    );
  }

  const hasScore = category.score !== null;
  const scoreText = hasScore ? roundHalfUp(category.score!, 1) : emptyScoreLabel(category);
  const osvSource = sources?.find((item) => item.source === 'osv' && item.scope === 'osv');

  return (
    <section className={styles.categoryCard} aria-label={`Категория: ${label}`}>
      <header className={styles.categoryHeader}>
        <h3 className={styles.categoryTitle}>{label}</h3>
        <div className={styles.categoryScore}>
          <span className={styles.metricCaption}>Балл</span>
          <span className={styles.metricValue}>{scoreText}</span>
        </div>
      </header>

      <div className={styles.categoryMeta}>
        <div>
          <strong>Состояние:</strong>{' '}
          <span>
            {hasScore && category.coverage.calculated < category.coverage.total
              ? STATUS_LABELS.partial
              : STATUS_LABELS[category.status]}
          </span>
        </div>
        <div>
          {reportDetails ? (
            `Рассчитано ${category.coverage.calculated} из ${category.coverage.total} метрик`
          ) : (
            <>
              <strong>Покрытие:</strong> {formatCoverage(category.coverage)}
            </>
          )}
        </div>
      </div>

      {category.reason && (
        <p className={styles.naReason} role="status">
          {category.reason.message}. {category.reason.cannot_conclude}
        </p>
      )}
      {reportDetails && category.id === 'security' && (
        <div className={styles.securityChecks} aria-label="Охват проверок безопасности">
          {category.metrics.map(({ input }) => {
            const linkedSources = (sources ?? []).filter(
              (source) =>
                source.metric_ids?.includes(input.id) ||
                (input.id === 'SEC_SECRETS' &&
                  source.source === 'git' &&
                  source.scope === 'secret_history'),
            );
            return (
              <div className={styles.securityCheck} key={input.id}>
                <strong>{METRIC_LABELS[input.id]}</strong>
                <span>{STATUS_LABELS[input.status]}</span>
                <span className={styles.hint}>{formatSample(input.sample, input.id)}</span>
                {input.reason && (
                  <span className={styles.naReason}>
                    {input.reason.message}. {input.reason.cannot_conclude}
                  </span>
                )}
                {linkedSources.map((source) => (
                  <div className={styles.securitySource} key={`${source.source}/${source.scope}`}>
                    <span>
                      Источник {source.source} · {source.scope}: {STATUS_LABELS[source.status]}
                    </span>
                    <span className={styles.hint}>{sourceSampleText(source)}</span>
                    {source.reason && (
                      <span className={styles.naReason}>
                        {source.reason.message}. {source.reason.cannot_conclude}
                      </span>
                    )}
                  </div>
                ))}
                {input.id === 'SEC_SECRETS' && input.status !== 'complete' && (
                  <span className={styles.hint}>
                    Неполный охват истории не подтверждает отсутствие секретов в непроверенных
                    ревизиях.
                  </span>
                )}
                {input.id === 'SEC_VULN_RISK' && input.raw_value === null && (
                  <span className={styles.hint}>
                    Неизвестная версия или уровень OSV не означают отсутствие уязвимости.
                  </span>
                )}
                {input.id === 'SEC_SAST' && (
                  <span className={styles.hint}>
                    Обнаружение конфигурации не подтверждает успешный запуск SAST.
                  </span>
                )}
              </div>
            );
          })}
        </div>
      )}
      {reportDetails && category.id === 'security' && (
        <section className={styles.securityCoverage} aria-label="Покрытие экосистем OSV">
          <h4>Зависимости, переданные на проверку OSV</h4>
          {(['npm', 'Maven', 'Go'] as const).map((ecosystem) => {
            const scope = `dependencies_${ecosystem.toLowerCase()}`;
            const source = sources?.find((item) => item.source === 'git' && item.scope === scope);
            const items = (dependencies ?? []).filter((item) => item.ecosystem === ecosystem);
            return (
              <div className={styles.securityEcosystem} key={ecosystem}>
                <strong>{ecosystem}</strong>
                {source ? (
                  <>
                    <span>
                      Источник {source.source} · {source.scope}: {STATUS_LABELS[source.status]}
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
                  <ul>
                    {items.map((item) => (
                      <li key={`${item.ecosystem}/${item.package}/${item.version}`}>
                        <strong>
                          {item.package}@{item.version}
                        </strong>
                        <ul>
                          {dependencyOrigins(item).map((origin) => (
                            <li key={`${origin.manifest_path}/${origin.version_source}`}>
                              Манифест: {origin.manifest_path} · источник версии:{' '}
                              {VERSION_SOURCE_LABELS[origin.version_source] ??
                                origin.version_source}
                              {isDeclaredVersion(origin.version_source) && (
                                <div className={styles.hint}>
                                  Версия объявлена в манифесте; фактический граф сборки не
                                  подтверждён.
                                </div>
                              )}
                            </li>
                          ))}
                        </ul>
                      </li>
                    ))}
                  </ul>
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
            <strong>Запросы OSV</strong>
            {osvSource ? (
              <>
                <span>
                  Источник {osvSource.source} · {osvSource.scope}: {STATUS_LABELS[osvSource.status]}
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

      {category.explanation && (
        <p id={explanationId} className={styles.hint}>
          {category.explanation}
        </p>
      )}
      {reportDetails &&
        !hasScore &&
        !category.reason &&
        !category.metrics.some((metric) => metric.input.reason) && (
          <p className={styles.naReason}>Причина недоступности не определена</p>
        )}

      {category.metrics.length > 0 ? (
        <details className={styles.categoryDetails} open={defaultOpen}>
          <summary className={styles.categorySummary}>
            {reportDetails ? 'Показать основания' : `Показатели (${category.metrics.length})`}
          </summary>
          <dl className={styles.fields}>
            {category.metrics.map((metric) => (
              <MetricDetails
                key={metric.input.id}
                metric={metric}
                sha={sha}
                anchor={reportDetails ? `metric-${metric.input.id}` : undefined}
                sharedExplanationId={
                  metric.input.id === 'TEST_PRESENCE' &&
                  explanation &&
                  explanation === metric.explanation?.trim()
                    ? explanationId
                    : undefined
                }
              />
            ))}
          </dl>
        </details>
      ) : (
        <p className={styles.hint}>Показатели отсутствуют</p>
      )}
    </section>
  );
}