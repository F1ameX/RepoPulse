import api from './api';
import type { AnalysisStatus, Report } from '../types';

export const repositoryService = {
  submitRepository: async (url: string): Promise<{ id: string }> => {
    const { data } = await api.post('/repositories', { url });
    return data;
  },

  getAnalysisStatus: async (id: string): Promise<AnalysisStatus> => {
    const { data } = await api.get(`/analysis/${id}`);
    return data;
  },

  getReport: async (id: string): Promise<Report> => {
    const { data } = await api.get(`/report/${id}`);
    return data;
  },
};
