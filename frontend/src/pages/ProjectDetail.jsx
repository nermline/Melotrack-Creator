import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { motion, Reorder, useDragControls } from 'framer-motion';
import api from '../api';
import { Glass, Button, TextInput, Spinner } from '../ui/kit';

const pageVariants = {
    initial: { opacity: 0, y: 18 },
    animate: { opacity: 1, y: 0, transition: { duration: 0.22, ease: [0.22, 0.61, 0.36, 1] } },
    exit:    { opacity: 0, y: -10, transition: { duration: 0.16, ease: 'easeIn' } },
};

function CategoryRow({ cat, index, canEdit, editing, editTitle, setEditTitle,
    onStartEdit, onSubmitEdit, onCancelEdit, onDelete, onOpen, onCommit }) {
    const controls = useDragControls();
    return (
        <Reorder.Item as="div" value={cat} dragListener={false} dragControls={controls}
            onDragEnd={() => onCommit(cat.id)} layout
            initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }}
            style={{
                borderRadius: 12, marginBottom: 8,
                border: '1px solid rgba(255,255,255,0.10)',
                background: 'rgba(255,255,255,0.045)',
                cursor: editing ? 'default' : 'pointer',
            }}
            onClick={!editing ? () => onOpen(cat.id) : undefined}>
            <div className="flex items-center gap-2 sm:gap-3 px-3 py-3">
                {editing ? (
                    <form onSubmit={(e) => onSubmitEdit(e, cat.id)} onClick={(e) => e.stopPropagation()}
                        className="flex gap-2 items-center flex-1 flex-wrap">
                        <TextInput autoFocus value={editTitle} onChange={(e) => setEditTitle(e.target.value)}
                            required className="flex-1 min-w-0" />
                        <Button type="submit" size="sm" variant="primary">OK</Button>
                        <Button type="button" size="sm" variant="ghost" onClick={onCancelEdit}>✕</Button>
                    </form>
                ) : (
                    <>
                        {canEdit && (
                            <div onPointerDown={(e) => controls.start(e)} onClick={(e) => e.stopPropagation()}
                                title="Перетягніть, щоб змінити порядок"
                                style={{ cursor: 'grab', color: 'var(--color-muted)', padding: '0 2px', touchAction: 'none', fontSize: 18 }}>
                                ⠿
                            </div>
                        )}
                        <span className="text-xs" style={{ color: 'var(--color-muted)', minWidth: 18 }}>{index + 1}</span>
                        <div className="flex-1 min-w-0">
                            <span className="font-semibold" style={{ color: '#fff' }}>{cat.title}</span>
                            <span className="text-xs ml-2" style={{ color: 'var(--color-muted)' }}>
                                {cat.items?.length ?? 0} питань
                            </span>
                        </div>
                        {canEdit && (
                            <Button size="sm" variant="ghost"
                                onClick={(e) => { e.stopPropagation(); onStartEdit(cat); }}>✎</Button>
                        )}
                        {canEdit && (
                            <Button size="sm" variant="danger"
                                onClick={(e) => { e.stopPropagation(); onDelete(cat.id); }}>🗑</Button>
                        )}
                    </>
                )}
            </div>
        </Reorder.Item>
    );
}

export default function ProjectDetail() {
    const { pid } = useParams();
    const navigate = useNavigate();

    const [project, setProject] = useState(null);
    const [categories, setCategories] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');

    const [editingTitle, setEditingTitle] = useState(false);
    const [newProjectTitle, setNewProjectTitle] = useState('');

    const [isCreating, setIsCreating] = useState(false);
    const [newCatTitle, setNewCatTitle] = useState('');
    const [createError, setCreateError] = useState('');

    const [editingCatId, setEditingCatId] = useState(null);
    const [editCatTitle, setEditCatTitle] = useState('');

    const role = localStorage.getItem('role') || '';
    const canEdit = role === 'admin' || role === 'editor';

    const catsRef = useRef(categories);
    useEffect(() => { catsRef.current = categories; }, [categories]);

    const sortCats = (arr) => [...arr].sort((a, b) => a.position - b.position);

    useEffect(() => {
        api.get(`/api/projects/${pid}`)
            .then((r) => { setProject(r.data); setCategories(sortCats(r.data.categories || [])); setNewProjectTitle(r.data.title); })
            .catch((err) => setError(err.response?.status === 404 ? 'Проєкт не знайдено' : 'Помилка завантаження'))
            .finally(() => setLoading(false));
    }, [pid]);

    const handleEditTitle = async (e) => {
        e.preventDefault();
        try {
            const r = await api.put(`/api/projects/${pid}`, { title: newProjectTitle });
            setProject((prev) => ({ ...prev, title: r.data.title }));
            setEditingTitle(false);
        } catch { alert('Не вдалося зберегти назву'); }
    };

    const handleDeleteProject = async () => {
        if (!confirm('Видалити проєкт разом з усіма категоріями та медіа?')) return;
        try { await api.delete(`/api/projects/${pid}`); navigate('/'); } catch { alert('Не вдалося видалити проєкт'); }
    };

    const handleCreateCat = async (e) => {
        e.preventDefault(); setCreateError('');
        try {
            const r = await api.post(`/api/projects/${pid}/categories`, { title: newCatTitle });
            setCategories((prev) => [...prev, r.data]);
            setNewCatTitle(''); setIsCreating(false);
        } catch (err) {
            setCreateError(err.response?.data?.error?.includes('unique') ? 'Назва вже зайнята' : 'Помилка створення');
        }
    };

    const handleEditCat = async (e, catId) => {
        e.preventDefault();
        try {
            const r = await api.put(`/api/projects/${pid}/categories/${catId}`, { title: editCatTitle });
            setCategories((prev) => prev.map((c) => c.id === catId ? { ...c, title: r.data.title } : c));
            setEditingCatId(null);
        } catch { alert('Не вдалося зберегти'); }
    };

    const handleDeleteCat = async (catId) => {
        if (!confirm('Видалити категорію з усіма питаннями?')) return;
        try {
            await api.delete(`/api/projects/${pid}/categories/${catId}`);
            setCategories((prev) => prev.filter((c) => c.id !== catId));
        } catch { alert('Не вдалося видалити категорію'); }
    };

    const commitCatReorder = async (id) => {
        const arr = catsRef.current;
        const newIndex = arr.findIndex((c) => c.id === id);
        const c = arr[newIndex];
        if (!c || newIndex === c.position) return;
        try { await api.put(`/api/projects/${pid}/categories/${id}`, { position: newIndex }); } catch {}
        try { const res = await api.get(`/api/projects/${pid}`); setCategories(sortCats(res.data.categories || [])); }
        catch {}
    };

    return (
        <motion.div variants={pageVariants} initial="initial" animate="animate" exit="exit">
            <div className="mx-auto max-w-5xl px-4 py-6">
                {loading ? (
                    <div className="py-4"><Spinner /> <span className="ml-2">Завантаження…</span></div>
                ) : error ? (
                    <div>
                        <p style={{ color: 'var(--color-danger)' }}>{error}</p>
                        <a href="/" onClick={(e) => { e.preventDefault(); navigate('/'); }}>← Назад</a>
                    </div>
                ) : (
                    <>
                        <p className="text-sm mb-3" style={{ color: 'var(--color-muted)' }}>
                            <a href="/" style={{ cursor: 'pointer' }}
                                onClick={(e) => { e.preventDefault(); navigate('/'); }}>← Всі проєкти</a>
                        </p>

                        <div className="flex items-center gap-3 flex-wrap mb-5">
                            {editingTitle ? (
                                <form onSubmit={handleEditTitle} className="flex gap-2 items-center flex-wrap">
                                    <TextInput autoFocus value={newProjectTitle}
                                        onChange={(e) => setNewProjectTitle(e.target.value)} required />
                                    <Button type="submit" size="sm" variant="primary">OK</Button>
                                    <Button type="button" size="sm" variant="ghost"
                                        onClick={() => setEditingTitle(false)}>✕</Button>
                                </form>
                            ) : (
                                <>
                                    <h1 className="text-2xl sm:text-3xl font-bold m-0" style={{ color: '#fff' }}>{project?.title}</h1>
                                    {canEdit && (
                                        <Button size="sm" variant="ghost" onClick={() => setEditingTitle(true)}>✎</Button>
                                    )}
                                </>
                            )}
                            <div className="ml-auto flex gap-2 flex-wrap">
                                <Button variant="primary" onClick={() => navigate(`/projects/${pid}/play`)}>▶ Показ</Button>
                                {canEdit && (
                                    <Button variant="danger" onClick={handleDeleteProject}>🗑 Видалити</Button>
                                )}
                            </div>
                        </div>

                        <div className="flex items-center justify-between gap-2 flex-wrap mb-3">
                            <h2 className="text-xl font-semibold m-0" style={{ color: '#fff' }}>Категорії</h2>
                            {canEdit && !isCreating && (
                                <Button size="sm" variant="primary"
                                    onClick={() => { setIsCreating(true); setCreateError(''); }}>
                                    + Категорія
                                </Button>
                            )}
                        </div>

                        {isCreating && (
                            <Glass className="p-3 mb-3">
                                <form onSubmit={handleCreateCat} className="flex gap-2 items-center flex-wrap">
                                    <TextInput autoFocus value={newCatTitle}
                                        onChange={(e) => setNewCatTitle(e.target.value)}
                                        placeholder="Назва категорії" required className="flex-1 min-w-0" />
                                    <Button type="submit" size="sm" variant="primary">Зберегти</Button>
                                    <Button type="button" size="sm" variant="ghost"
                                        onClick={() => setIsCreating(false)}>Скасувати</Button>
                                </form>
                                {createError && (
                                    <p className="mt-2 text-sm" style={{ color: 'var(--color-danger)' }}>{createError}</p>
                                )}
                            </Glass>
                        )}

                        {categories.length === 0 ? (
                            <p style={{ color: 'var(--color-muted)' }}>Категорій ще немає.</p>
                        ) : (
                            <>
                                {canEdit && (
                                    <p className="text-xs mb-2" style={{ color: 'var(--color-muted)' }}>
                                        перетягуйте ⠿, щоб змінити порядок
                                    </p>
                                )}
                                <Reorder.Group as="div" axis="y" values={categories} onReorder={setCategories}>
                                    {categories.map((cat, idx) => (
                                        <CategoryRow key={cat.id} cat={cat} index={idx} canEdit={canEdit}
                                            editing={editingCatId === cat.id}
                                            editTitle={editCatTitle} setEditTitle={setEditCatTitle}
                                            onStartEdit={(c) => { setEditingCatId(c.id); setEditCatTitle(c.title); }}
                                            onSubmitEdit={handleEditCat}
                                            onCancelEdit={() => setEditingCatId(null)}
                                            onDelete={handleDeleteCat}
                                            onOpen={(id) => navigate(`/projects/${pid}/categories/${id}`)}
                                            onCommit={commitCatReorder} />
                                    ))}
                                </Reorder.Group>
                            </>
                        )}
                    </>
                )}
            </div>
        </motion.div>
    );
}
