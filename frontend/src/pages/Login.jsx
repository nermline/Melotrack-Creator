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
            // gin-jwt за замовчуванням очікує поля username та password
            const response = await api.post('/login', {
                username: username,
                password: password
            });

            // Зберігаємо токен у LocalStorage
            localStorage.setItem('token', response.data.token);
            
            // Переходимо на сторінку проєктів
            navigate('/');
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