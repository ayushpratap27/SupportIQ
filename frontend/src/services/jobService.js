import api from './api';

const jobService = {
  list(params = {}) {
    return api.get('/jobs', { params });
  },

  getById(id) {
    return api.get(`/jobs/${id}`);
  },

  retry(id) {
    return api.post(`/jobs/${id}/retry`);
  },

  statistics() {
    return api.get('/jobs/statistics');
  },
};

export default jobService;
