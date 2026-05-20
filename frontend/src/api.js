import axios from 'axios';

const api = axios.create({
    baseURL: 'http://localhost:8080', // Зміни на порт свого бекенду
    headers: {
        'Content-Type': 'application/json',
    },
});

// 1. Автоматично додаємо токен до кожного запиту
api.interceptors.request.use((config) => {
    const token = localStorage.getItem('token');
    if (token) {
        config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
});

// 2. Автоматично оновлюємо токен (Refresh), якщо він протух
api.interceptors.response.use(
    (response) => response, // Якщо все ок — просто повертаємо відповідь
    async (error) => {
        const originalRequest = error.config;

        // Якщо помилка 401 (Unauthorized) і ми ще не намагалися оновити токен
        if (error.response && error.response.status === 401 && !originalRequest._retry) {
            originalRequest._retry = true; // Ставимо прапорець, щоб не зациклитись

            try {
                // Робимо запит на оновлення токена (стандартний шлях gin-jwt)
                const response = await axios.get('http://localhost:8080/refresh_token', {
                    headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
                });
                
                // Зберігаємо новий токен
                localStorage.setItem('token', response.data.token);
                
                // Повторюємо оригінальний запит з новим токеном
                originalRequest.headers.Authorization = `Bearer ${response.data.token}`;
                return api(originalRequest);
            } catch (refreshError) {
                // Якщо і refresh протух — викидаємо юзера на логін
                localStorage.removeItem('token');
                window.location.href = '/login';
                return Promise.reject(refreshError);
            }
        }
        return Promise.reject(error);
    }
);

export default api;