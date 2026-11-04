import styles from '@/components/Page/Page.module.css';
export function AnalysisTitle({ repository }: { repository?: { owner: string; name: string } }) {
  return (
    <h1 className={styles.title}>
      {repository ? (
        <>
          {repository.owner}/<wbr />
          {repository.name}
        </>
      ) : (
        'Анализ репозитория'
      )}
    </h1>
  );
}
