import api from './api';

const BASE = '/integrations';

export const integrationService = {
  list: () => api.get(BASE),

  create: (data) => api.post(BASE, data),

  update: (id, data) => api.put(`${BASE}/${id}`, data),

  delete: (id) => api.delete(`${BASE}/${id}`),

  test: (id) => api.post(`${BASE}/${id}/test`),

  listEvents: (id) => api.get(`${BASE}/${id}/events`),

  getTicketIntegrations: (ticketId) => api.get(`/tickets/${ticketId}/integrations`),

  createJira: (ticketId) => api.post(`/tickets/${ticketId}/create-jira`),

  createLinear: (ticketId) => api.post(`/tickets/${ticketId}/create-linear`),

  createGitHub: (ticketId) => api.post(`/tickets/${ticketId}/create-github-issue`),
};

export default integrationService;
