import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api';

export default function Login() {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const navigate = useNavigate();

    const handleLogin = async (e) => {
            e.preventDefault();
            setError('');

            try {
                const response = await api.post('/login', {
                    username: username,
                    password: password
                });

                console.log("Відповідь сервера при логіні:", response.data);

                // [ЗМІНЕНО]: Тепер беремо access_token замість token
                const actualToken = response.data.access_token;

                if (actualToken && typeof actualToken === 'string') {
                    localStorage.setItem('token', actualToken);
                    // Дістаємо роль з payload JWT, щоб показувати відповідні дії показу.
                    try {
                        const payload = JSON.parse(atob(actualToken.split('.')[1]));
                        localStorage.setItem('role', payload.role || '');
                    } catch {
                        localStorage.removeItem('role');
                    }
                    navigate('/');
                } else {
                    setError('Сервер не повернув токен. Перевір консоль (F12).');
                }
            } catch (err) {
                setError('Невірний логін або пароль');
                console.error('Помилка входу:', err);
            }
        };

    return (
        <div style={{ padding: '20px' }}>
            <h1>Вхід у Melotrack</h1>
            {error && <p style={{ color: 'red' }}>{error}</p>}
            
            <form onSubmit={handleLogin} style={{ display: 'flex', flexDirection: 'column', width: '200px', gap: '10px' }}>
                <label>Логін:</label>
                <input 
                    type="text" 
                    value={username} 
                    onChange={(e) => setUsername(e.target.value)} 
                    required 
                />
                
                <label>Пароль:</label>
                <input 
                    type="password" 
                    value={password} 
                    onChange={(e) => setPassword(e.target.value)} 
                    required 
                />
                
                <button type="submit">Увійти</button>
            </form>
        </div>
    );
}