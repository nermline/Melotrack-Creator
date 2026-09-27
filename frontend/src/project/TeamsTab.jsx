import { useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Check, Copy, Plus, QrCode, Trash2, Users } from 'lucide-react';
import { api } from '../lib/api';
import { joinUrl, teams as teamsLabel } from '../lib/format';
import { Button, Empty, InlineEdit, QR, Toggle } from '../ui';
import { useFeedback } from '../ui/feedbackContext';

export default function TeamsTab({ store }) {
    const { project } = store;
    const [name, setName] = useState('');
    const [copied, setCopied] = useState(false);
    const { toast, confirm } = useFeedback();
    const url = joinUrl(project.join_code);

    const setOpen = async (open) => {
        try {
            const p = await api.patch(`/api/projects/${project.id}`, { registration_open: open });
            store.dispatch({ type: 'patch', patch: { registration_open: p.registration_open } });
        } catch (e) {
            toast(e.message, 'bad');
        }
    };

    const add = async (e) => {
        e.preventDefault();
        if (!name.trim()) return;
        try {
            await api.post(`/api/projects/${project.id}/teams`, { name });
            setName('');
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const update = async (team, body) => {
        try {
            await api.patch(`/api/teams/${team.id}`, body);
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const remove = async (team) => {
        const ok = await confirm({
            title: 'Видалити команду?',
            message: `«${team.name}» і всі її бали буде видалено.`,
            confirmLabel: 'Видалити',
            danger: true,
        });
        if (!ok) return;
        try {
            await api.del(`/api/teams/${team.id}`);
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const copy = async () => {
        try {
            await navigator.clipboard.writeText(url);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
        } catch {
            toast('Не вдалося скопіювати', 'bad');
        }
    };

    return (
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_340px]">
            <section className="min-w-0">
                <div className="mb-3 flex items-end justify-between gap-3">
                    <div>
                        <div className="eyebrow">Учасники</div>
                        <h2 className="m-0 mt-1 font-display text-xl font-bold">{teamsLabel(project.teams.length)}</h2>
                    </div>
                </div>
                <form onSubmit={add} className="mb-4 flex gap-2">
                    <input
                        className="field"
                        placeholder="Назва команди"
                        maxLength={40}
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                    />
                    <Button type="submit" variant="primary" icon={Plus} disabled={!name.trim()}>
                        Додати
                    </Button>
                </form>

                {project.teams.length === 0 ? (
                    <div className="panel">
                        <Empty icon={Users} title="Команд ще немає">
                            Додайте їх тут або відкрийте реєстрацію — капітани відскановують QR на екрані й впишуть назву
                            самі.
                        </Empty>
                    </div>
                ) : (
                    <div className="panel divide-y divide-line">
                        <AnimatePresence initial={false}>
                            {project.teams.map((t, i) => (
                                <motion.div
                                    key={t.id}
                                    layout
                                    initial={{ opacity: 0, backgroundColor: 'rgba(255,200,92,0.15)' }}
                                    animate={{ opacity: 1, backgroundColor: 'rgba(0,0,0,0)' }}
                                    exit={{ opacity: 0 }}
                                    transition={{ duration: 0.6 }}
                                    className="flex items-center gap-3 px-4 py-2.5"
                                >
                                    <span className="w-6 text-center font-display text-sm font-bold text-faint">{i + 1}</span>
                                    <div className="min-w-0 flex-1">
                                        <InlineEdit value={t.name} maxLength={40} onSave={(v) => v && update(t, { name: v })} className="font-semibold" />
                                    </div>
                                    <Button size="sm" variant="ghost" icon={Trash2} className="text-faint hover:text-bad" onClick={() => remove(t)} />
                                </motion.div>
                            ))}
                        </AnimatePresence>
                    </div>
                )}
            </section>

            <aside className="panel self-start p-5">
                <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 font-bold">
                        <QrCode size={18} /> Самореєстрація
                    </div>
                    <Toggle checked={project.registration_open} onChange={setOpen} />
                </div>
                <p className="mt-2 mb-4 text-sm text-dim">
                    {project.registration_open
                        ? 'Відкрито: QR-код показується на вітальному екрані, капітани реєструються з телефонів.'
                        : 'Закрито: додавайте команди вручну або відкрийте реєстрацію перед початком.'}
                </p>
                <div className={project.registration_open ? '' : 'opacity-40'}>
                    <QR value={url} size={200} className="mx-auto" />
                    <div className="mt-3 flex items-center gap-2">
                        <input className="field py-1.5 font-mono text-xs" readOnly value={url} onFocus={(e) => e.target.select()} />
                        <Button size="sm" icon={copied ? Check : Copy} onClick={copy} title="Скопіювати посилання" />
                    </div>
                </div>
            </aside>
        </div>
    );
}
