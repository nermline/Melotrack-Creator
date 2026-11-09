import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { motion } from 'framer-motion';
import { Lock } from 'lucide-react';
import { api } from '../lib/api';
import { Button, Spinner } from '../ui';
import Logo from '../ui/Logo';

// Why "Sign in with Google" sent the person back here (?error=…).
const GOOGLE_ERRORS = {
    denied: 'Цей акаунт Google не має доступу до Мелотреку. Попросіть адміністратора додати ваш email або увійдіть іншим акаунтом.',
    cancelled: 'Вхід скасовано.',
    expired: 'Вхід тривав задовго. Спробуйте ще раз.',
    failed: 'Не вдалося увійти через Google. Спробуйте ще раз.',
};

function GoogleLogo() {
    return (
        <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
            <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z" />
            <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z" />
            <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z" />
            <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z" />
        </svg>
    );
}

export default function Login() {
    const [mode, setMode] = useState(null);
    const [params] = useSearchParams();
    const next = params.get('next');
    const target = next && next.startsWith('/') && !next.startsWith('//') ? next : '/';

    useEffect(() => {
        api.get('/api/auth/config')
            .then((c) => setMode(c.mode))
            .catch(() => setMode('password'));
    }, []);

    return (
        <div className="grid min-h-screen place-items-center p-4">
            <motion.div
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
                {mode === null && (
                    <div className="py-4 text-center text-dim">
                        <Spinner size={20} />
                    </div>
                )}
                {mode === 'google' && <GoogleSignIn target={target} error={GOOGLE_ERRORS[params.get('error')]} />}
                {mode === 'password' && <PasswordSignIn target={target} />}
            </motion.div>
        </div>
    );
}

function GoogleSignIn({ target, error }) {
    return (
        <>
            {error && <p className="mt-0 mb-4 rounded-xl bg-bad/10 px-3 py-2.5 text-sm text-bad">{error}</p>}
            <a
                href={`/api/auth/google/start?next=${encodeURIComponent(target)}`}
                className="flex h-11 w-full items-center justify-center gap-3 rounded-xl border border-[#747775] bg-white px-4 text-[15px] font-semibold text-[#1f1f1f] no-underline transition-colors hover:bg-[#f2f2f2]"
            >
                <GoogleLogo />
                Увійти через Google
            </a>
        </>
    );
}

function PasswordSignIn({ target }) {
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const navigate = useNavigate();

    const submit = async (e) => {
        e.preventDefault();
        setBusy(true);
        setError('');
        try {
            await api.post('/api/login', { password });
            navigate(target, { replace: true });
        } catch (err) {
            setError(err.message);
            setPassword('');
        } finally {
            setBusy(false);
        }
    };

    return (
        <form onSubmit={submit}>
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
        </form>
    );
}
