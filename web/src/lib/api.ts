import type { Provider, Target, Input, RunSummary, RunDetail, CallRaw } from './types';

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  });

  if (!res.ok) {
    let errMsg = `Request failed: ${res.status}`;
    try {
      const errObj = await res.json();
      if (errObj && errObj.error) {
        errMsg = errObj.error;
      }
    } catch (e) {
      // Ignore
    }
    throw new Error(errMsg);
  }

  return res.json();
}

export const api = {
  // Health
  getHealth: () => fetchJSON<{ ok: boolean; version: string; build_time?: string; git_commit?: string }>('/api/health'),

  // Providers
  listProviders: async () => (await fetchJSON<Provider[]>('/api/providers')) || [],
  createProvider: (data: Partial<Provider> & { api_key?: string }) =>
    fetchJSON<Provider>('/api/providers', { method: 'POST', body: JSON.stringify(data) }),
  updateProvider: (id: number, data: Partial<Provider> & { api_key?: string }) =>
    fetchJSON<Provider>(`/api/providers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteProvider: (id: number) =>
    fetchJSON<{ ok: boolean }>(`/api/providers/${id}`, { method: 'DELETE' }),
  testProvider: (id: number, modelName?: string) =>
    fetchJSON<{ ok: boolean; http_status: number; total_ms: number; error?: string }>(
      `/api/providers/${id}/test`,
      { method: 'POST', body: JSON.stringify({ model_name: modelName }) }
    ),
  fetchProviderModels: (id: number) =>
    fetchJSON<{ ok: boolean; http_status?: number; models: string[]; error?: string }>(
      `/api/providers/${id}/models`
    ),

  // Targets
  listTargets: async () => (await fetchJSON<Target[]>('/api/targets')) || [],
  createTarget: (data: Partial<Target>) =>
    fetchJSON<Target>('/api/targets', { method: 'POST', body: JSON.stringify(data) }),
  updateTarget: (id: number, data: Partial<Target>) =>
    fetchJSON<Target>(`/api/targets/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTarget: (id: number) =>
    fetchJSON<{ ok: boolean }>(`/api/targets/${id}`, { method: 'DELETE' }),
  duplicateTarget: (id: number) =>
    fetchJSON<Target>(`/api/targets/${id}/duplicate`, { method: 'POST' }),

  // Preview
  preview: (targetId: number, input: Input) =>
    fetchJSON<{ body: string; headers: Record<string, string> }>('/api/preview', {
      method: 'POST',
      body: JSON.stringify({ target_id: targetId, input }),
    }),

  // Runs
  startRun: (input: Input, targetIds: number[], timeoutS: number, note: string, signal?: AbortSignal) =>
    fetch('/api/runs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input, target_ids: targetIds, timeout_s: timeoutS, note }),
      signal,
    }),

  cancelCall: (callId: number) =>
    fetchJSON<{ ok: boolean }>(`/api/calls/${callId}/cancel`, { method: 'POST' }),

  listRuns: async (limit = 50, before?: number) => {
    let url = `/api/runs?limit=${limit}`;
    if (before) url += `&before=${before}`;
    return (await fetchJSON<RunSummary[]>(url)) || [];
  },
  getRun: (id: number) => fetchJSON<RunDetail>(`/api/runs/${id}`),
  deleteRun: (id: number) => fetchJSON<{ ok: boolean }>(`/api/runs/${id}`, { method: 'DELETE' }),

  // Calls
  getCallRaw: (callId: number) => fetchJSON<CallRaw>(`/api/calls/${callId}/raw`),
  exportCall: async (callId: number, format: 'curl' | 'python') => {
    const res = await fetch(`/api/calls/${callId}/export?format=${format}`);
    if (!res.ok) {
      throw new Error(`Export failed: ${res.status}`);
    }
    return res.text();
  },
};
