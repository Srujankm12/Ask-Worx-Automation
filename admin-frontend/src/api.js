import axios from 'axios';

const BASE_URL = import.meta.env.VITE_API_URL || 'https://askworx-bot-production.up.railway.app';

const api = axios.create({
  baseURL: `${BASE_URL}/api`,
});

// Create a separate instance for login to use the full URL easily
const authApi = axios.create({
  baseURL: BASE_URL,
});


// For simplicity in this demo, we'll just check for a token in localStorage
// Add ngrok-skip-browser-warning header for development
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('askworx_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  if (config.baseURL && config.baseURL.includes('ngrok')) {
    config.headers['ngrok-skip-browser-warning'] = 'true';
  }
  return config;
});

export const login = (password) => authApi.post('/api/login', { password });

export const getStats = () => api.get('/stats');
export const getLeads = (params) => api.get('/leads', { params });
export const updateLeadStatus = (id, status) => api.post('/leads/update-status', { id, status });
export const getCallbacks = () => api.get('/callbacks');
export const markCallbackDone = (id) => api.post('/callbacks/mark-done', { id });
export const getContacts = () => api.get('/contacts');
export const saveContact = (data) => api.post('/contacts', data);
export const deleteContact = (id) => api.delete(`/contacts/${id}`);
export const toggleOptOut = (id, optOut) => api.post(`/contacts/${id}/opt-out`, { opt_out: optOut });
export const getMessages = (params) => api.get('/messages', { params });
export const getChatHistory = (phone) => api.get(`/messages/${phone}`);
export const sendMessage = (phone, message) => api.post('/send-message', { phone, message });

// Campaign Management
export const getCampaigns = (params) => api.get('/campaigns', { params });
export const createCampaign = (data) => api.post('/campaigns', data);
export const deleteCampaign = (id) => api.delete(`/campaigns/${id}`);
export const getCampaignAnalytics = (id) => api.get(`/campaigns/${id}/analytics`);

// Employee Management
export const getEmployees = (params) => api.get('/employees', { params });
export const addEmployee = (data) => api.post('/employees', data);
export const deleteEmployee = (id) => api.delete(`/employees/${id}`);
export const getAttendance = (params) => api.get('/attendance', { params });
export const getLeaveRequests = (params) => api.get('/leave-requests', { params });
export const updateLeaveStatus = (id, status) => api.post('/leave-requests/update-status', { id, status });
export const createReminder = (data) => api.post('/reminders', data);
export const getRemindersHistory = (params) => api.get('/reminders/history', { params });
export const sendAnnouncement = (data) => api.post('/announcements', data);

export const uploadImage = (file) => {
  const formData = new FormData();
  formData.append('file', file);
  return api.post('/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  });
};

export const getSettings = () => api.get('/settings');
export const updateSettings = (data) => api.post('/settings', data);

export const getFaqs = () => api.get('/faqs');
export const saveFaq = (data) => api.post('/faqs', data);
export const deleteFaq = (id) => api.delete(`/faqs/${id}`);

export default api;
