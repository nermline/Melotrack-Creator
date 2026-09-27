import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import { CopyPlus, Disc3, ListMusic, Plus, Trash2, Users } from 'lucide-react';
import { api } from '../lib/api';
import { categories, fmtDate, songs, teams } from '../lib/format';
import { Button, Empty, Modal, Spinner } from '../ui';
import { useFeedback } from '../ui/feedbackContext';
import TopBar from '../ui/TopBar';
import { THEMES } from '../show/themes';

function TitleDialog({ open, title, initial = '', submitLabel, onClose, onSubmit }) {
    return (
        <Modal open={open} onClose={onClose} title={title} width={460}>
            <TitleForm initial={initial} submitLabel={submitLabel} onClose={onClose} onSubmit={onSubmit} />
        </Modal>
    );
}

function TitleForm({ initial, submitLabel, onClose, onSubmit }) {
    const [value, setValue] = useState(initial);
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);

    const submit = async (e) => {
        e.preventDefault();
        setBusy(true);
        setError('');
        try {
            await onSubmit(value.trim());
        } catch (err) {
            setError(err.message);
        } finally {
            setBusy(false);
        }
    };
    return (
        <form onSubmit={submit}>
                <label className="label" htmlFor="ptitle">
                    Назва
                </label>
                <input
                    id="ptitle"
                    className="field"
                    autoFocus
                    maxLength={120}
                    placeholder="Мелотрек 27 вересня"
                    value={value}
                    onChange={(e) => setValue(e.target.value)}
                />
                {error && <p className="mt-2 mb-0 text-sm text-bad">{error}</p>}
                <div className="mt-5 flex justify-end gap-2">
                    <Button variant="ghost" onClick={onClose}>
                        Скасувати
                    </Button>
                    <Button type="submit" variant="primary" loading={busy} disabled={!value.trim()}>
                        {submitLabel}
                    </Button>
                </div>
        </form>
    );
}

export default function Projects() {
    const [list, setList] = useState(null);
    const [error, setError] = useState('');
    const [creating, setCreating] = useState(false);
    const [duplicating, setDuplicating] = useState(null);
    const navigate = useNavigate();
    const { confirm, toast } = useFeedback();

    const load = () =>
        api
            .get('/api/projects')
            .then(setList)
            .catch((e) => setError(e.message));
    useEffect(() => {
        load();
    }, []);

    const create = async (title) => {
        const p = await api.post('/api/projects', { title });
        navigate(`/p/${p.id}`);
    };

    const duplicate = async (title) => {
        const p = await api.post(`/api/projects/${duplicating.id}/duplicate`, { title });
        setDuplicating(null);
        toast('Копію створено — пісні та кліпи вже на місці');
        navigate(`/p/${p.id}`);
    };

    const remove = async (p) => {
        const ok = await confirm({
            title: 'Видалити мелотрек?',
            message: `«${p.title}» буде видалено разом з категоріями, піснями, командами та балами. Це не можна скасувати.`,
            confirmLabel: 'Видалити',
            danger: true,
        });
        if (!ok) return;
        try {
            await api.del(`/api/projects/${p.id}`);
            setList((l) => l.filter((x) => x.id !== p.id));
        } catch (e) {
            toast(e.message, 'bad');
        }
    };

    return (
        <>
            <TopBar />
            <main className="mx-auto max-w-[1100px] px-4 py-8">
                <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
                    <div>
                        <div className="eyebrow">Усі події</div>
                        <h1 className="m-0 mt-1 font-display text-2xl font-bold sm:text-3xl">Мелотреки</h1>
                    </div>
                    <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
                        Новий мелотрек
                    </Button>
                </div>

                {error && <p className="text-bad">{error}</p>}
                {!list && !error && (
                    <div className="py-10 text-center text-dim">
                        <Spinner />
                    </div>
                )}
                {list && list.length === 0 && (
                    <div className="panel">
                        <Empty
                            icon={Disc3}
                            title="Ще жодного мелотреку"
                            action={
                                <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
                                    Створити перший
                                </Button>
                            }
                        >
                            Створіть мелотрек, додайте категорії й пісні з YouTube — програма сама завантажить і
                            підготує фрагменти.
                        </Empty>
                    </div>
                )}
                {list && list.length > 0 && (
                    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                        {list.map((p, i) => {
                            const theme = THEMES[p.theme] || THEMES.neon;
                            return (
                                <motion.div
                                    key={p.id}
                                    initial={{ opacity: 0, y: 10 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    transition={{ delay: i * 0.03 }}
                                    className="panel group relative cursor-pointer overflow-hidden p-5 transition-colors hover:border-line-strong"
                                    onClick={() => navigate(`/p/${p.id}`)}
                                >
                                    <div
                                        className="absolute inset-x-0 top-0 h-1"
                                        style={{ background: theme.swatch }}
                                        aria-hidden="true"
                                    />
                                    <div className="text-xs text-faint">{fmtDate(p.created_at)}</div>
                                    <div className="mt-1 mb-4 line-clamp-2 text-lg font-bold">{p.title}</div>
                                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-dim">
                                        <span className="inline-flex items-center gap-1.5">
                                            <ListMusic size={14} /> {categories(p.categories)}, {songs(p.items)}
                                        </span>
                                        <span className="inline-flex items-center gap-1.5">
                                            <Users size={14} /> {teams(p.teams)}
                                        </span>
                                    </div>
                                    <div
                                        className="absolute top-3 right-3 flex gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100"
                                        onClick={(e) => e.stopPropagation()}
                                    >
                                        <Button
                                            size="sm"
                                            variant="ghost"
                                            icon={CopyPlus}
                                            title="Створити копію як шаблон"
                                            onClick={() => setDuplicating(p)}
                                        />
                                        <Button
                                            size="sm"
                                            variant="ghost"
                                            icon={Trash2}
                                            title="Видалити"
                                            onClick={() => remove(p)}
                                        />
                                    </div>
                                </motion.div>
                            );
                        })}
                    </div>
                )}
            </main>

            <TitleDialog
                open={creating}
                title="Новий мелотрек"
                submitLabel="Створити"
                onClose={() => setCreating(false)}
                onSubmit={create}
            />
            <TitleDialog
                open={!!duplicating}
                title="Копія мелотреку"
                initial={duplicating ? `${duplicating.title} (копія)` : ''}
                submitLabel="Створити копію"
                onClose={() => setDuplicating(null)}
                onSubmit={duplicate}
            />
        </>
    );
}
