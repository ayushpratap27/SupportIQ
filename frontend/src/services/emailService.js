import api from './api';

const emailService = {
  // Account management (admin)
  listAccounts() {
    return api.get('/email/accounts');
  },
  createAccount(data) {
    return api.post('/email/accounts', data);
  },
  updateAccount(id, data) {
    return api.put(`/email/accounts/${id}`, data);
  },
  deleteAccount(id) {
    return api.delete(`/email/accounts/${id}`);
  },
  testConnection(id, protocol = 'smtp') {
    return api.post(`/email/accounts/${id}/test?protocol=${protocol}`);
  },

  // Monitor
  getMonitor() {
    return api.get('/email/monitor');
  },

  // Sync
  triggerSync() {
    return api.post('/email/sync');
  },

  // Ticket emails
  getTicketEmails(ticketId) {
    return api.get(`/tickets/${ticketId}/emails`);
  },
  sendEmail(ticketId, data) {
    return api.post(`/tickets/${ticketId}/send-email`, data);
  },
};

export default emailService;
