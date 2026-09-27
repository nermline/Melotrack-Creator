import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { motion } from 'framer-motion';
import { Lock } from 'lucide-react';
import { api } from '../lib/api';
import { Button } from '../ui';
import Logo from '../ui/Logo';

export default function Login() {
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const navigate = useNavigate();
    const [params] = useSearchParams();

    const submit = async (e) => {
        e.preventDefault();
        setBusy(true);
        setError('');
        try {
            await api.post('/api/login', { password });
            const next = params.get('next');
            navigate(next && next.startsWith('/') ? next : '/', { replace: true });
        } catch (err) {
            setError(err.message);
            setPassword('');
        } finally {
            setBusy(false);
        }
    };

    return (
        <div className="grid min-h-screen place-items-center p-4">
            <motion.form
                onSubmit={submit}
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ type: 'spring', stiffness: 260, damping: 26 }}
                className="panel w-full max-w-sm p-7 shadow-2xl"
            >
                <div className="mb-7 flex flex-col items-center text-center">
                    <Logo size={52} />
                    <h1 className="mt-4 mb-1 font-display text-2xl font-bold">Мелотрек</h1>
                    <p className="m-0 text-sm text-dim">Простір організаторів</p>
                </div>
                <label className="label" htmlFor="pw">
                    Пароль
                </label>
                <div className="relative">
                    <Lock size={16} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-faint" />
                    <input
                        id="pw"
                        type="password"
                        className="field pl-9"
                        autoFocus
                        autoComplete="current-password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                    />
                </div>
                {error && <p className="mt-2 mb-0 text-sm text-bad">{error}</p>}
                <Button type="submit" variant="primary" className="mt-5 w-full" loading={busy} disabled={!password}>
                    Увійти
                </Button>
            </motion.form>
        </div>
    );
}
