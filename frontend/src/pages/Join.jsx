import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { AnimatePresence, motion } from 'framer-motion';
import confetti from 'canvas-confetti';
import '@fontsource-variable/playfair-display';
import '@fontsource-variable/oswald';
import { api } from '../lib/api';
import { Spinner } from '../ui';
import { Backdrop } from '../show/visuals';
import '../show/show.css';

// Public page the QR code on the welcome screen points to: a team captain
// types the team name and it appears on the big screen right away.
export default function Join() {
    const { code } = useParams();
    const [info, setInfo] = useState(null);
    const [error, setError] = useState('');
    const [name, setName] = useState('');
    const [busy, setBusy] = useState(false);
    const [done, setDone] = useState(() => {
        try {
            return localStorage.getItem(`mt-join-${code}`) || '';
        } catch {
            return '';
        }
    });

    useEffect(() => {
        api.get(`/api/public/join/${code}`)
            .then((d) => {
                setInfo(d);
                document.title = `Реєстрація — ${d.title}`;
            })
            .catch((e) => setError(e.message));
    }, [code]);

    const submit = async (e) => {
        e.preventDefault();
        setBusy(true);
        setError('');
        try {
            const res = await api.post(`/api/public/join/${code}`, { name });
            setDone(res.name);
            try {
                localStorage.setItem(`mt-join-${code}`, res.name);
            } catch {
                /* private mode */
            }
            setInfo((i) => ({ ...i, teams: [...(i.teams || []), res.name] }));
            confetti({ particleCount: 120, spread: 80, origin: { y: 0.7 } });
        } catch (err) {
            setError(err.message);
        } finally {
            setBusy(false);
        }
    };

    const theme = info?.theme || 'neon';

    return (
        <div className="themed fixed inset-0 overflow-hidden" data-theme={theme}>
            <Backdrop theme={theme} dim />
            <div className="relative z-10 flex h-full flex-col items-center overflow-y-auto px-5 py-10">
                {!info && !error && <Spinner size={24} />}
                {!info && error && <p className="mt-20 text-center">{error}</p>}
                {info && (
                    <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} className="w-full max-w-sm">
                        <div className="text-center text-xs font-extrabold tracking-[0.22em] uppercase" style={{ color: 'var(--a2)' }}>
                            Музичний квіз
                        </div>
                        <h1 className="display mt-2 mb-8 text-center text-4xl" style={{ textShadow: 'var(--glow)' }}>
                            {info.title}
                        </h1>
                        <AnimatePresence mode="wait">
                            {done ? (
                                <motion.div
                                    key="done"
                                    initial={{ opacity: 0, scale: 0.9 }}
                                    animate={{ opacity: 1, scale: 1 }}
                                    className="rounded-3xl border p-6 text-center backdrop-blur"
                                    style={{ background: 'var(--card)', borderColor: 'var(--card-line)' }}
                                >
                                    <div className="text-sm" style={{ color: 'var(--dim)' }}>
                                        Ви в грі!
                                    </div>
                                    <div className="display mt-2 text-3xl" style={{ color: 'var(--a2)' }}>
                                        {done}
                                    </div>
                                    <p className="mt-4 mb-0 text-sm" style={{ color: 'var(--dim)' }}>
                                        Шукайте назву своєї команди на великому екрані. Не забудьте підписати бланк відповідей саме так.
                                    </p>
                                    <button
                                        type="button"
                                        className="mt-5 text-xs underline opacity-60"
                                        onClick={() => {
                                            setDone('');
                                            setName('');
                                        }}
                                    >
                                        Зареєструвати ще одну команду
                                    </button>
                                </motion.div>
                            ) : !info.open ? (
                                <motion.p key="closed" className="text-center" style={{ color: 'var(--dim)' }}>
                                    Реєстрацію зараз закрито. Зверніться до організаторів.
                                </motion.p>
                            ) : (
                                <motion.form key="form" onSubmit={submit} className="flex flex-col gap-3" exit={{ opacity: 0 }}>
                                    <label className="text-sm font-semibold" style={{ color: 'var(--dim)' }} htmlFor="team">
                                        Назва команди
                                    </label>
                                    <input
                                        id="team"
                                        autoFocus
                                        maxLength={40}
                                        value={name}
                                        onChange={(e) => setName(e.target.value)}
                                        placeholder="Наприклад, «Шукачі мелодій»"
                                        className="h-14 rounded-2xl border px-4 text-lg outline-none"
                                        style={{ background: 'var(--card)', borderColor: 'var(--card-line)', color: 'var(--ink)' }}
                                    />
                                    {error && <p className="m-0 text-sm text-bad">{error}</p>}
                                    <button
                                        type="submit"
                                        disabled={busy || !name.trim()}
                                        className="h-14 rounded-2xl text-lg font-bold disabled:opacity-40"
                                        style={{ background: 'linear-gradient(90deg, var(--a1), var(--a2))', color: 'var(--bg)' }}
                                    >
                                        {busy ? <Spinner /> : 'Зареєструватись'}
                                    </button>
                                </motion.form>
                            )}
                        </AnimatePresence>
                        {info.teams?.length > 0 && (
                            <div className="mt-10">
                                <div className="mb-2 text-center text-xs" style={{ color: 'var(--dim)' }}>
                                    Уже зареєструвались
                                </div>
                                <div className="flex flex-wrap justify-center gap-2">
                                    {info.teams.map((t) => (
                                        <span
                                            key={t}
                                            className="rounded-full border px-3 py-1 text-sm"
                                            style={{ background: 'var(--card)', borderColor: 'var(--card-line)', color: t === done ? 'var(--a2)' : undefined }}
                                        >
                                            {t}
                                        </span>
                                    ))}
                                </div>
                            </div>
                        )}
                    </motion.div>
                )}
            </div>
        </div>
    );
}
