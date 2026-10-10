import { useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Plus, Trash2, Users } from 'lucide-react';
import { api } from '../lib/api';
import { teams as teamsLabel } from '../lib/format';
import { Button, Empty, InlineEdit } from '../ui';
import { useFeedback } from '../ui/feedbackContext';

export default function TeamsTab({ store }) {
    const { project } = store;
    const [name, setName] = useState('');
    const { toast, confirm } = useFeedback();

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

    return (
        <div className="max-w-3xl">
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
                            Додайте назви команд — вони зʼявляться на вітальному екрані й у таблиці оцінювання.
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

        </div>
    );
}
