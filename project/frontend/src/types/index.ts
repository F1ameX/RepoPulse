export interface Repository {
  id: string;
  url: string;
  createdAt: string;
}

export interface AnalysisStatus {
  id: string;
  status: 'pending' | 'running' | 'done' | 'failed';
  progress?: number;
}

export interface Report {
  id: string;
  repositoryUrl: string;
  createdAt: string;
  summary: string;
}
