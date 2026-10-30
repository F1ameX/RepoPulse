import type { ReactNode } from 'react';
import type { Repository, Versions, Window } from '@/types';
import { formatUtc } from '@/lib/analysisView';
import styles from './Analysis.module.css';

type HeaderData = {
  repository: Repository;
  analysis_id: string;
  report_id?: string | null;
  completed_at?: string | null;
  assessment_at?: string | null;
  window?: Window | null;
  versions?: Partial<Versions> | null;
};

function known(value: string | null | undefined): value is string {
  if (!value?.trim()) return false;
  return !['не определено', 'не определен', 'не определена'].includes(
    value.trim().toLowerCase().replaceAll('ё', 'е'),
  );
}

export function ReportHeader({
  report,
  label = 'Шапка отчёта',
}: {
  report: HeaderData;
  label?: string;
}) {
  const { repository } = report;
  const url = repository.canonical_url;
  const safeUrl = /^https:\/\/github\.com\/[A-Za-z0-9-]+\/[A-Za-z0-9_.-]+$/.test(url);
  const sha = repository.commit_sha;
  const hasSha = sha !== null && /^[a-f0-9]{40}$/i.test(sha);
  const completed = formatUtc(report.completed_at);
  const assessment = formatUtc(report.assessment_at);
  const from = formatUtc(report.window?.from);
  const until = formatUtc(report.window?.until);
  const fields: [string, ReactNode][] = [];
  const add = (name: string, value: string | null | undefined, node?: ReactNode) => {
    fields.push([name, known(value) ? (node ?? value) : 'Не определено']);
  };
  add('Платформа', repository.platform === 'github' ? 'GitHub' : null);
  add(
    'Репозиторий',
    known(repository.owner) && known(repository.name)
      ? `${repository.owner}/${repository.name}`
      : null,
  );
  add(
    'Ссылка',
    safeUrl ? url : null,
    <a href={url} target="_blank" rel="noopener noreferrer">
      {url}
    </a>,
  );
  add('Анализ', report.analysis_id);
  if (known(report.report_id) && report.report_id !== report.analysis_id)
    add('Отчёт', report.report_id);
  add('Завершён', completed);
  add('Момент оценки', assessment);
  add('Период оценки', known(from) && known(until) ? `[${from}, ${until})` : null);
  add('Ветка', repository.default_branch);
  add(
    'Commit',
    hasSha ? sha : null,
    hasSha && safeUrl ? (
      <a href={`${url}/commit/${sha}`} target="_blank" rel="noopener noreferrer">
        {sha}
      </a>
    ) : (
      sha
    ),
  );
  const versions: [string, string | null | undefined][] = [
    ['Методика', report.versions?.methodology],
    ['Профиль сборщика', report.versions?.collector_profile],
    ['Каталог рекомендаций', report.versions?.recommendation_catalog],
  ];
  return (
    <section aria-label={label} className={styles.report}>
      <dl className={styles.fields}>
        {fields.map(([name, value]) => (
          <div className={styles.field} key={name}>
            <dt>{name}:</dt>
            <dd>{value}</dd>
          </div>
        ))}
        <div className={styles.versions}>
          <dt>Версии:</dt>
          <dd>
            <dl>
              {versions.map(([name, value]) => (
                <div className={styles.field} key={name}>
                  <dt>{name}:</dt>
                  <dd>{known(value) ? value : 'Не определено'}</dd>
                </div>
              ))}
            </dl>
          </dd>
        </div>
      </dl>
    </section>
  );
}
