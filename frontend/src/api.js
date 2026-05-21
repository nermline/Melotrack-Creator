import axios from 'axios';

// Порожній baseURL → усі запити відносні до поточного origin.
// У продакшені фронтенд роздається з того ж Go-сервера (:8080), а у dev
// Vite проксує /api, /login, /refresh, /media тощо на бекенд (див. vite.config.js).
const api = axios.create({
    baseURL: '',
    headers: {
        'Content-Type': 'application/json',
    },
});

api.interceptors.request.use((config) => {
    const token = localStorage.getItem('token');
    // ЗАХИСТ: додаємо перевірку на рядки 'undefined' та 'null'
    if (token && token !== 'undefined' && token !== 'null') {
        config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
});

api.interceptors.response.use(
    (response) => response,
    async (error) => {
        const originalRequest = error.config;

        if (error.response && error.response.status === 401 && !originalRequest._retry) {
            
            // ЗАХИСТ ВІД ЦИКЛУ: Якщо сам запит /refresh повернув 401/400, 
            // не намагаємося рефрешити його знову. Просто викидаємо на логін.
            if (originalRequest.url === '/refresh') {
                localStorage.removeItem('token');
                window.location.href = '/login';
                return Promise.reject(error);
            }

            originalRequest._retry = true;
            try {
                const response = await axios.get('/refresh', {
                    headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
                });
                
                // [ЗМІНЕНО]: Дістаємо новий токен з поля access_token
                const newToken = response.data.access_token;
                
                localStorage.setItem('token', newToken);
                originalRequest.headers.Authorization = `Bearer ${newToken}`;
                return api(originalRequest);
            } catch (refreshError) {
                localStorage.removeItem('token');
                window.location.href = '/login';
                return Promise.reject(refreshError);
            }
        }
        return Promise.reject(error);
    }
);

export default api;