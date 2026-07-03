import apiClient from './api'

export const authService = {
  register: (data) => apiClient.post('/api/v1/auth/register', data),
  login: (data) => apiClient.post('/api/v1/auth/login', data),
  logout: () => apiClient.post('/api/v1/auth/logout'),
  getMe: () => apiClient.get('/api/v1/auth/me'),
}
