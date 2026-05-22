import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import api from '../api';
import { Glass, Button, Field, TextInput, Spinner } from '../ui/kit';

export default function Login() {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const navigate = useNavigate();

    const handleLogin = async (e) => {
        e.preventDefault();
        setError(''); setBusy(true);
        try {
            const response = await api.post('/api/login', { username, password });
            const actualToken = response.data.access_token;
            if (actualToken && typeof actualToken === 'string') {
                localStorage.setItem('token', actualToken);
                try {
                    const payload = JSON.parse(atob(actualToken.split('.')[1]));
                    localStorage.setItem('role', payload.role || '');
                } catch { localStorage.removeItem('role'); }
                navigate('/');
            } else {
                setError('Сервер не повернув токен.');
            }
        } catch {
            setError('Невірний логін або пароль');
        } finally { setBusy(false); }
    };

    return (
        <div className="min-h-screen grid place-items-center p-4">
            <motion.div initial={{ opacity: 0, y: 18, scale: 0.98 }} animate={{ opacity: 1, y: 0, scale: 1 }}
                transition={{ type: 'spring', stiffness: 280, damping: 26 }} style={{ width: 360, maxWidth: '100%' }}>
                <Glass className="p-7">
                    <div className="text-center mb-6">
                        <div className="text-2xl font-bold" style={{ color: '#fff' }}>Melotrack</div>
                        <div className="text-sm" style={{ color: 'var(--color-muted)' }}>Вхід для організаторів</div>
                    </div>
                    <form onSubmit={handleLogin} className="grid gap-4">
                        <Field label="Логін">
                            <TextInput value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus />
                        </Field>
                        <Field label="Пароль" error={error}>
                            <TextInput type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
                        </Field>
                        <Button type="submit" variant="primary" disabled={busy} className="w-full">
                            {busy ? <Spinner /> : 'Увійти'}
                        </Button>
                    </form>
                </Glass>
            </motion.div>
        </div>
    );
}
