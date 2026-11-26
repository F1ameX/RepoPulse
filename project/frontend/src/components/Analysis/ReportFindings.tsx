import { useId, type MouseEvent, type ReactNode } from 'react';
import { Button, Message } from '@/components';
import { isSafeUrl } from '@/lib/metricFormatting';
import {
  CATEGORY_LABELS,
  COMPLEXITY_LABELS,
  findingsScope,
  PRIORITY_LABELS,
  SEVERITY_LABELS,
  sortFindings,
  sortRecommendations,
  validFindings,
  validRecommendations,
} from '@/lib/reportFindings';
import type { Finding, Report } from '@/types';
import { EvidenceList } from './EvidenceList';
import styles from './Analysis.module.css';
import listStyles from './ReportFindings.module.css';

function ReportLink({ target, children }: { target: string; children: ReactNode }) {
  const follow = (event: MouseEvent<HTMLAnchorElement>) => {
    const element = document.getElementById(target);
    if (!element) return;
    event.preventDefault();
    const details = element.closest('details');
    if (details) details.open = true;
    element.focus({ preventScroll: true });
    element.scrollIntoView?.({ block: 'start' });
  };
  return (
    <a href={`#${target}`} onClick={follow}>
      {children}
    </a>
  );
}

const VULNERABILITY_SEVERITY_LABELS: Record<string, string> = {
  critical: 'Критический',
  high: 'Высокий',
  medium: 'Средний',
  low: 'Низкий',
};

function versionSourceLabel(source: string): string {
  const labels: Record<string, string> = {
    npm_lock_v2: 'npm lock v2',
    npm_lock_v3: 'npm lock v3',
    maven_declaration: 'Декларация Maven',
    maven_dependency_management: 'dependencyManagement Maven',
    go_require_declaration: 'Декларация require в go.mod',
  };
  return labels[source] ?? source;
}

function isDeclaredBuildVersion(source: string): boolean {
  return (
    source === 'maven_declaration' ||
    source === 'maven_dependency_management' ||
    source === 'go_require_declaration'
  );
}

function FindingEvidence({ finding, report }: { finding: Finding; report: Report }) {
  const categories = Array.isArray(report.categories) ? report.categories : [];
  const metrics = categories.flatMap((item) => (Array.isArray(item?.metrics) ? item.metrics : []));
  const category = categories.filter((item) => item?.id === finding.category);
  const matches = category.flatMap((item) =>
    Array.isArray(item.metrics)
      ? item.metrics.filter((metric) => metric?.input?.id === finding.rule_id)
      : [],
  );
  const metric = matches.length === 1 ? matches[0] : undefined;
  const available = metrics.flatMap((item) =>
    Array.isArray(item?.input?.evidence) ? item.input.evidence : [],
  );
  const ids = [...new Set(finding.evidence_ids)];
  const evidence = ids.flatMap((id) => {
    const matches = available.filter((item) => item?.id === id);
    const first = matches[0];
    return first && matches.every((item) => JSON.stringify(item) === JSON.stringify(first))
      ? [first]
      : [];
  });
  const safeEvidence =
    finding.rule_id === 'SEC_SECRETS'
      ? evidence.map((item) => ({
          ...item,
          secret_match_type: item.secret_match_type || 'Тип совпадения не определён',
        }))
      : evidence;
  return (
    <>
      {metric && <ReportLink target={`metric-${finding.rule_id}`}>Показать основание</ReportLink>}
      {(!metric || evidence.length !== ids.length) && (
        <p role="alert">Не удалось загрузить связанные основания</p>
      )}
      <EvidenceList
        evidence={safeEvidence}
        limited={metric?.input.evidence_limited ?? false}
        total={metric?.input.evidence_total ?? null}
        reason={metric?.input.reason ?? null}
        limit={finding.rule_id === 'TEST_PRESENCE' ? 10 : undefined}
        secret={finding.rule_id === 'SEC_SECRETS'}
      />
      {finding.rule_id === 'SEC_VULN_RISK' && finding.vulnerabilities.length > 0 && (
        <>
          {metric &&
          !['unavailable', 'not_applicable'].includes(metric.input.status) &&
          metric.input.raw_value !== null &&
          metric.input.raw_value !== undefined ? (
            <p>
              <strong>Risk:</strong> {String(metric.input.raw_value)}
            </p>
          ) : (
            <p>
              <strong>Risk:</strong> не рассчитан
            </p>
          )}
          <ul>
            {finding.vulnerabilities.map((item) => {
              const dependencies = Array.isArray(report.dependencies) ? report.dependencies : [];
              const dependency = dependencies.find(
                (candidate) =>
                  candidate.ecosystem === item.ecosystem &&
                  candidate.package === item.package &&
                  candidate.version === item.version,
              );
              const origins = dependency?.origins?.length
                ? dependency.origins
                : dependency
                  ? [
                      {
                        manifest_path: dependency.manifest_path,
                        version_source: dependency.version_source,
                      },
                    ]
                  : [];
              return (
                <li
                  key={`${item.vulnerability_id}/${item.ecosystem}/${item.package}/${item.version}`}
                >
                  <strong>{item.vulnerability_id}</strong> · {item.ecosystem}/{item.package}@
                  {item.version}
                  <div>
                    Уровень:{' '}
                    {item.severity
                      ? VULNERABILITY_SEVERITY_LABELS[item.severity] ?? item.severity
                      : 'Неизвестен'}
                  </div>
                  {origins.length === 0 && (
                    <div className={styles.hint}>
                      Сведения о манифесте и источнике версии недоступны.
                    </div>
                  )}
                  {origins.map((origin) => (
                    <div key={`${origin.manifest_path}/${origin.version_source}`}>
                      Манифест: {origin.manifest_path} · Источник версии:{' '}
                      {versionSourceLabel(origin.version_source)}
                      {isDeclaredBuildVersion(origin.version_source) && (
                        <div className={styles.hint}>
                          Версия объявлена в манифесте; фактический граф сборки не подтверждён.
                        </div>
                      )}
                    </div>
                  ))}
                  {isSafeUrl(item.evidence_url) && (
                    <a href={item.evidence_url} target="_blank" rel="noopener noreferrer">
                      Источник OSV
                    </a>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}
    </>
  );
}

export function ReportFindings({
  report,
  onReload,
  explanationMode,
}: {
  report: Report;
  onReload?: () => void;
  explanationMode?: ReactNode;
}) {
  const prefix = useId();
  const findingsValid = validFindings(report.findings);
  const recommendationsValid =
    validRecommendations(report.recommendations) &&
    (!findingsValid || report.findings.length > 0 || report.recommendations.length === 0);
  const findings = findingsValid ? sortFindings(report.findings) : [];
  const recommendations = recommendationsValid ? sortRecommendations(report.recommendations) : [];
  const scope = findingsScope(report);
  const findingAnchor = (id: string) => `${prefix}-finding-${encodeURIComponent(id)}`;
  const recommendationAnchor = (id: string) => `${prefix}-recommendation-${encodeURIComponent(id)}`;
  const incomplete = ['partial', 'unavailable'].includes(scope);
  const limitations = (
    <p className={styles.hint}>
      Часть проверок неполная, недоступна или неприменима.{' '}
      <ReportLink target="report-limitations">Ограничения анализа</ReportLink>
    </p>
  );
  const error = (name: string) => (
    <Message
      variant="error"
      actions={
        onReload && (
          <Button variant="secondary" onClick={onReload}>
            Повторить загрузку
          </Button>
        )
      }
    >
      Не удалось загрузить {name}
    </Message>
  );
  const empty = (kind: 'findings' | 'recommendations') => {
    if (kind === 'recommendations') return <p>Для этого отчёта рекомендации не сформированы</p>;
    if (scope === 'error') return error('список проблем');
    if (scope === 'not_applicable') return <p>Применимые проверки отсутствуют</p>;
    if (scope === 'unavailable') return <p>Недостаточно данных для выявления проблем</p>;
    if (scope === 'partial') {
      return (
        <p>
          По доступным результатам проблемы не обнаружены, но часть проверок неполная или неприменима;
          отсутствие проблем не подтверждено.
        </p>
      );
    }
    return <p>По выполненным проверкам проблемы не обнаружены</p>;
  };

  return (
    <>
      <section className={styles.categoryList} aria-label="Проблемы">
        <h2 className={styles.sectionTitle}>Проблемы</h2>
        {findings.length > 0 && scope === 'error' && error('сведения о полноте списка')}
        {findingsValid && incomplete && limitations}
        {!findingsValid ? (
          error('список проблем')
        ) : !findings.length ? (
          empty('findings')
        ) : (
          <ul className={listStyles.list}>
            {findings.map((finding) => (
              <li key={finding.id} id={findingAnchor(finding.id)} tabIndex={-1}>
                <h3>
                  {finding.rule_id === 'SEC_SECRETS' ? 'Потенциальный секрет' : finding.title}
                </h3>
                <p className={styles.hint}>
                  {CATEGORY_LABELS[finding.category]} · Критичность —{' '}
                  {SEVERITY_LABELS[finding.severity]}
                </p>
                {finding.rule_id === 'SEC_SECRETS' && (
                  <p>Действительность реквизитов не проверялась</p>
                )}
                {finding.rule_id === 'SEC_SECRETS' ? (
                  <p>
                    Совпадение с правилом поиска секрета. Значение и исходная строка не отображаются.
                  </p>
                ) : (
                  <>
                    <p>{finding.description}</p>
                    <p>
                      <strong>Возможное последствие</strong> — {finding.possible_consequence}
                    </p>
                  </>
                )}
                <FindingEvidence finding={finding} report={report} />
                <div className={listStyles.links}>
                  {recommendations
                    .filter((item) => item.finding_id === finding.id)
                    .map((item) => (
                      <ReportLink key={item.id} target={recommendationAnchor(item.id)}>
                        К рекомендации
                      </ReportLink>
                    ))}
                  {(!recommendationsValid ||
                    !recommendations.some((item) => item.finding_id === finding.id)) && (
                    <span>Связанная рекомендация недоступна</span>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className={styles.categoryList} aria-label="Рекомендации">
        <h2 className={styles.sectionTitle}>Рекомендации</h2>
        {recommendations.length > 0 && explanationMode}
        {recommendationsValid && incomplete && limitations}
        {!recommendationsValid ? (
          error('рекомендации')
        ) : !recommendations.length ? (
          empty('recommendations')
        ) : (
          <ul className={listStyles.list}>
            {recommendations.map((recommendation) => {
              const finding = findings.find((item) => item.id === recommendation.finding_id);
              return (
                <li
                  key={recommendation.id}
                  id={recommendationAnchor(recommendation.id)}
                  tabIndex={-1}
                >
                  <h3>{recommendation.title}</h3>
                  <p className={styles.hint}>
                    Приоритет — {PRIORITY_LABELS[recommendation.priority]} · Сложность —{' '}
                    {COMPLEXITY_LABELS[recommendation.complexity]}
                  </p>
                  <p>
                    <strong>Действие</strong> — {recommendation.action}
                  </p>
                  <p>
                    <strong>Ожидаемая польза</strong> — {recommendation.expected_benefit}
                  </p>
                  <p>{recommendation.explanation}</p>
                  {finding ? (
                    <ReportLink target={findingAnchor(finding.id)}>Показать основание</ReportLink>
                  ) : (
                    <p role="alert">Основание рекомендации недоступно</p>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </>
  );
}
