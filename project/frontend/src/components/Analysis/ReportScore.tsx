import type { MouseEvent } from 'react';
import { Button, Message } from '@/components';
import { roundHalfUp } from '@/lib/metricFormatting';
import { isReportCoverage, isReportScore } from '@/lib/reportPresentation';
import type { Report } from '@/types';
import styles from './Analysis.module.css';

export function ReportScore({ report, onReload }: { report: Report; onReload?: () => void }) {
  const { overall_score: score, overall_reason: reason, coverage } = report;
  const valid =
    isReportScore(score) &&
    isReportCoverage(coverage, 12) &&
    typeof report.summary === 'string' &&
    (score === null
      ? reason &&
        typeof reason.code === 'string' &&
        typeof reason.message === 'string' &&
        reason.message.trim().length > 0 &&
        typeof reason.cannot_conclude === 'string'
      : reason === null);
  const incomplete =
    valid &&
    (coverage.calculated < 12 ||
      coverage.partial > 0 ||
      coverage.unavailable > 0 ||
      coverage.not_applicable > 0 ||
      !Array.isArray(report.sources) ||
      report.sources.some((source) => source?.status !== 'complete' || source.reason) ||
      !Array.isArray(report.categories) ||
      report.categories.length !== 5 ||
      report.categories.some((category) => category?.status !== 'complete' || category.reason));
  const showLimitations = (event: MouseEvent<HTMLAnchorElement>) => {
    const target = document.getElementById('report-limitations');
    if (!target) return;
    event.preventDefault();
    target.focus({ preventScroll: true });
    target.scrollIntoView?.({ block: 'start' });
  };

  return (
    <section className={styles.categoryList} aria-label="Repo Health Score">
      <h2 className={styles.sectionTitle}>Repo Health Score</h2>
      {!valid ? (
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
          Не удалось загрузить оценку
        </Message>
      ) : (
        <>
          {score === null ? (
            <>
              <p>Недостаточно данных для общей оценки</p>
              <p className={styles.naReason}>
                {reason?.message}
                {reason?.cannot_conclude && `. ${reason.cannot_conclude}`}
              </p>
            </>
          ) : (
            <p className={styles.overallScore}>{roundHalfUp(score, 1).replace('.', ',')} / 100</p>
          )}
          {report.summary && <p>{report.summary}</p>}
          <p>
            Рассчитано метрик: {coverage.calculated} из 12 (
            {roundHalfUp(coverage.percentage, 1).replace('.', ',')}%)
          </p>
          {incomplete && (
            <p>
              {score !== null && 'Оценка основана на неполных данных. '}
              <a href="#report-limitations" onClick={showLimitations}>
                Ограничения анализа
              </a>
            </p>
          )}
        </>
      )}
      <p className={styles.hint}>
        Score — первичная техническая оценка, не гарантия качества или безопасности
      </p>
    </section>
  );
}
