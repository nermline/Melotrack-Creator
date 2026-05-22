import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import api from '../api';
import { Glass, Button, TextInput, Spinner } from '../ui/kit';

const pageVariants = {
    initial: { opacity: 0, y: 18 },
    animate: { opacity: 1, y: 0, transition: { duration: 0.22, ease: [0.22, 0.61, 0.36, 1] } },
    exit:    { opacity: 0, y: -10, transition: { duration: 0.16, ease: 'easeIn' } },
};

export default function Projects() {
    const [projects, setProjects] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');

    const [isCreating, setIsCreating] = useState(false);
    const [newTitle, setNewTitle] = useState('');
    const [createError, setCreateError] = useState('');

    const [editingId, setEditingId] = useState(null);
    const [editTitle, setEditTitle] = useState('');

    const navigate = useNavigate();
    const role = localStorage.getItem('role') || '';
    const canEdit = role === 'admin' || role === 'editor';

    useEffect(() => {
        api.get('/api/projects')
            .then((r) => setProjects(r.data || []))
            .catch(() => setError('Помилка завантаження проєктів'))
            .finally(() => setLoading(false));
    }, []);

    const handleLogout = async () => {
        try { await api.post('/api/logout'); } catch { /* ignore */ }
        localStorage.removeItem('token'); localStorage.removeItem('role');
        navigate('/login');
    };

    const handleCreate = async (e) => {
        e.preventDefault(); setCreateError('');
        try {
            const r = await api.post('/api/projects', { title: newTitle });
            setProjects((prev) => [r.data, ...prev]);
            setNewTitle(''); setIsCreating(false);
        } catch (err) {
            const msg = err.response?.data?.error;
            setCreateError(msg === 'title is already taken' ? 'Назва вже зайнята' : 'Не вдалося створити');
        }
    };

    const handleEdit = async (e, id) => {
        e.preventDefault();
        try {
            const r = await api.put(`/api/projects/${id}`, { title: editTitle });
            setProjects((prev) => prev.map((p) => p.id === id ? { ...p, title: r.data.title } : p));
            setEditingId(null);
        } catch { alert('Не вдалося оновити'); }
    };

    const handleDelete = async (id) => {
        if (!confirm('Видалити проєкт разом з усіма категоріями та медіа?')) return;
        try {
            await api.delete(`/api/projects/${id}`);
            setProjects((prev) => prev.filter((p) => p.id !== id));
        } catch { alert('Не вдалося видалити проєкт'); }
    };

    return (
        <motion.div variants={pageVariants} initial="initial" animate="animate" exit="exit">
            <div className="mx-auto max-w-5xl px-4 py-6">
                <div className="flex items-center justify-between gap-2 flex-wrap mb-5">
                    <h1 className="text-2xl sm:text-3xl font-bold m-0" style={{ color: '#fff' }}>Проєкти</h1>
                    <div className="flex items-center gap-2">
                        <span className="text-xs px-2 py-1 rounded-full"
                            style={{ background: 'rgba(255,255,255,0.08)', color: 'var(--color-muted)' }}>
                            {role || 'гість'}
                        </span>
                        <Button variant="ghost" size="sm" onClick={handleLogout}>Вийти</Button>
                    </div>
                </div>

                {error && <p style={{ color: 'var(--color-danger)' }}>{error}</p>}

                {canEdit && (isCreating ? (
                    <Glass className="p-4 mb-4">
                        <form onSubmit={handleCreate} className="flex gap-2 items-center flex-wrap">
                            <TextInput autoFocus value={newTitle} onChange={(e) => setNewTitle(e.target.value)}
                                placeholder="Назва проєкту" required className="flex-1 min-w-0" />
                            <Button type="submit" variant="primary">Зберегти</Button>
                            <Button type="button" variant="ghost" onClick={() => setIsCreating(false)}>Скасувати</Button>
                        </form>
                        {createError && <p className="mt-2 text-sm" style={{ color: 'var(--color-danger)' }}>{createError}</p>}
                    </Glass>
                ) : (
                    <Button variant="primary" className="mb-4"
                        onClick={() => { setIsCreating(true); setCreateError(''); }}>
                        + Новий проєкт
                    </Button>
                ))}

                {loading ? (
                    <div className="py-4"><Spinner /> <span className="ml-2">Завантаження…</span></div>
                ) : projects.length === 0 ? (
                    <p style={{ color: 'var(--color-muted)' }}>Проєктів ще немає.</p>
                ) : (
                    <div className="grid gap-3">
                        <AnimatePresence initial={false}>
                            {projects.map((proj) => {
                                const isEditing = editingId === proj.id;
                                return (
                                    <motion.div
                                        key={proj.id}
                                        layout
                                        initial={{ opacity: 0, y: 8 }}
                                        animate={{ opacity: 1, y: 0 }}
                                        exit={{ opacity: 0, scale: 0.97 }}
                                        whileHover={!isEditing ? { y: -2, transition: { duration: 0.13 } } : undefined}
                                        onClick={!isEditing ? () => navigate(`/projects/${proj.id}`) : undefined}
                                        style={!isEditing ? { cursor: 'pointer' } : undefined}
                                    >
                                        <Glass className="p-4 flex items-center gap-3">
                                            {isEditing ? (
                                                <form onSubmit={(e) => handleEdit(e, proj.id)}
                                                    className="flex gap-2 items-center flex-1">
                                                    <TextInput autoFocus value={editTitle}
                                                        onChange={(e) => setEditTitle(e.target.value)} required />
                                                    <Button type="submit" size="sm" variant="primary">OK</Button>
                                                    <Button type="button" size="sm" variant="ghost"
                                                        onClick={() => setEditingId(null)}>✕</Button>
                                                </form>
                                            ) : (
                                                <>
                                                    <div className="flex-1 min-w-0">
                                                        <div className="font-semibold truncate" style={{ color: '#fff' }}>
                                                            {proj.title}
                                                        </div>
                                                        <div className="text-xs" style={{ color: 'var(--color-muted)' }}>
                                                            {new Date(proj.created_at).toLocaleDateString()}
                                                        </div>
                                                    </div>
                                                    {canEdit && (
                                                        <Button size="sm" variant="ghost"
                                                            onClick={(e) => { e.stopPropagation(); setEditingId(proj.id); setEditTitle(proj.title); }}>
                                                            ✎
                                                        </Button>
                                                    )}
                                                    {canEdit && (
                                                        <Button size="sm" variant="danger"
                                                            onClick={(e) => { e.stopPropagation(); handleDelete(proj.id); }}>
                                                            🗑
                                                        </Button>
                                                    )}
                                                </>
                                            )}
                                        </Glass>
                                    </motion.div>
                                );
                            })}
                        </AnimatePresence>
                    </div>
                )}
            </div>
        </motion.div>
    );
}
