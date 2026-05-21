import { useEffect, useState } from 'react';
import { useNavigate, useParams, Link } from 'react-router-dom';
import api from '../api';

export default function ProjectDetail() {
    const { pid } = useParams();
    const navigate = useNavigate();

    const [project, setProject] = useState(null);
    const [categories, setCategories] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');

    // Edit project title
    const [editingTitle, setEditingTitle] = useState(false);
    const [newProjectTitle, setNewProjectTitle] = useState('');
    const [titleError, setTitleError] = useState('');

    // Create category
    const [isCreating, setIsCreating] = useState(false);
    const [newCatTitle, setNewCatTitle] = useState('');
    const [createError, setCreateError] = useState('');

    // Edit category
    const [editingCatId, setEditingCatId] = useState(null);
    const [editCatTitle, setEditCatTitle] = useState('');
    const [editCatError, setEditCatError] = useState('');

    useEffect(() => {
        api.get(`/api/projects/${pid}`)
            .then(r => {
                setProject(r.data);
                setCategories(r.data.categories || []);
                setNewProjectTitle(r.data.title);
            })
            .catch(err => {
                if (err.response?.status === 404) setError('Проєкт не знайдено');
                else setError('Помилка завантаження');
            })
            .finally(() => setLoading(false));
    }, [pid]);

    // ---- Project title ----
    const handleEditTitle = async (e) => {
        e.preventDefault();
        setTitleError('');
        try {
            const r = await api.put(`/api/projects/${pid}`, { title: newProjectTitle });
            setProject(prev => ({ ...prev, title: r.data.title }));
            setEditingTitle(false);
        } catch (err) {
            const msg = err.response?.data?.error;
            setTitleError(msg === 'title is already taken' ? 'Назва вже зайнята' : 'Помилка збереження');
        }
    };

    const handleDeleteProject = async () => {
        if (!confirm('Видалити проєкт разом з усіма категоріями та медіа?')) return;
        try {
            await api.delete(`/api/projects/${pid}`);
            navigate('/');
        } catch {
            alert('Не вдалося видалити проєкт');
        }
    };

    // ---- Categories ----
    const handleCreateCat = async (e) => {
        e.preventDefault();
        setCreateError('');
        try {
            const r = await api.post(`/api/projects/${pid}/categories`, { title: newCatTitle });
            setCategories(prev => [...prev, r.data]);
            setNewCatTitle('');
            setIsCreating(false);
        } catch (err) {
            const msg = err.response?.data?.error;
            setCreateError(msg?.includes('unique') ? 'Назва вже зайнята' : 'Помилка створення');
        }
    };

    const startEditCat = (cat) => {
        setEditingCatId(cat.id);
        setEditCatTitle(cat.title);
        setEditCatError('');
    };

    const handleEditCat = async (e, catId) => {
        e.preventDefault();
        setEditCatError('');
        try {
            const r = await api.put(`/api/projects/${pid}/categories/${catId}`, { title: editCatTitle });
            setCategories(prev => prev.map(c => c.id === catId ? { ...c, title: r.data.title } : c));
            setEditingCatId(null);
        } catch (err) {
            const msg = err.response?.data?.error;
            setEditCatError(msg?.includes('unique') ? 'Назва вже зайнята' : 'Помилка збереження');
        }
    };

    const handleDeleteCat = async (catId) => {
        if (!confirm('Видалити категорію з усіма питаннями?')) return;
        try {
            await api.delete(`/api/projects/${pid}/categories/${catId}`);
            setCategories(prev => prev.filter(c => c.id !== catId));
        } catch {
            alert('Не вдалося видалити категорію');
        }
    };

    const handleMoveCat = async (cat, direction) => {
        const sorted = [...categories].sort((a, b) => a.position - b.position);
        const idx = sorted.findIndex(c => c.id === cat.id);
        const newPos = direction === 'up' ? idx - 1 : idx + 1;
        if (newPos < 0 || newPos >= sorted.length) return;
        try {
            const r = await api.put(`/api/projects/${pid}/categories/${cat.id}`, { position: newPos });
            // Reload categories to get updated positions
            const res = await api.get(`/api/projects/${pid}`);
            setCategories(res.data.categories || []);
        } catch {
            alert('Не вдалося змінити порядок');
        }
    };

    if (loading) return <p>Завантаження...</p>;
    if (error) return <div><p style={{ color: 'red' }}>{error}</p><Link to="/">← Назад</Link></div>;

    const sortedCats = [...categories].sort((a, b) => a.position - b.position);

    return (
        <div style={{ padding: '16px', maxWidth: '700px' }}>
            {/* Breadcrumb */}
            <p><Link to="/">← Всі проєкти</Link></p>

            {/* Project header */}
            {editingTitle ? (
                <form onSubmit={handleEditTitle} style={{ marginBottom: '8px' }}>
                    <input
                        autoFocus
                        value={newProjectTitle}
                        onChange={e => setNewProjectTitle(e.target.value)}
                        required
                        style={{ fontSize: '1.2em' }}
                    />
                    {' '}
                    <button type="submit">OK</button>
                    {' '}
                    <button type="button" onClick={() => setEditingTitle(false)}>✕</button>
                    {titleError && <span style={{ color: 'red', marginLeft: '8px' }}>{titleError}</span>}
                </form>
            ) : (
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
                    <h1 style={{ margin: 0 }}>{project?.title}</h1>
                    <button onClick={() => { setEditingTitle(true); setTitleError(''); }}>✏️</button>
                    <button onClick={handleDeleteProject} style={{ color: 'red' }}>🗑 Видалити проєкт</button>
                </div>
            )}

            <hr />
            <h2>Категорії</h2>

            {/* Create category */}
            {!isCreating ? (
                <button onClick={() => { setIsCreating(true); setCreateError(''); }}>+ Нова категорія</button>
            ) : (
                <form onSubmit={handleCreateCat} style={{ marginBottom: '12px' }}>
                    <input
                        autoFocus
                        value={newCatTitle}
                        onChange={e => setNewCatTitle(e.target.value)}
                        placeholder="Назва категорії"
                        required
                    />
                    {' '}
                    <button type="submit">Зберегти</button>
                    {' '}
                    <button type="button" onClick={() => setIsCreating(false)}>Скасувати</button>
                    {createError && <span style={{ color: 'red', marginLeft: '8px' }}>{createError}</span>}
                </form>
            )}

            <hr />

            {sortedCats.length === 0 ? (
                <p>Категорій ще немає.</p>
            ) : (
                <ul style={{ listStyle: 'none', padding: 0 }}>
                    {sortedCats.map((cat, idx) => (
                        <li key={cat.id} style={{ borderBottom: '1px solid #ccc', padding: '8px 0' }}>
                            {editingCatId === cat.id ? (
                                <form onSubmit={e => handleEditCat(e, cat.id)} style={{ display: 'inline' }}>
                                    <input
                                        autoFocus
                                        value={editCatTitle}
                                        onChange={e => setEditCatTitle(e.target.value)}
                                        required
                                    />
                                    {' '}
                                    <button type="submit">OK</button>
                                    {' '}
                                    <button type="button" onClick={() => setEditingCatId(null)}>✕</button>
                                    {editCatError && <span style={{ color: 'red', marginLeft: '8px' }}>{editCatError}</span>}
                                </form>
                            ) : (
                                <span>
                                    <button
                                        onClick={() => handleMoveCat(cat, 'up')}
                                        disabled={idx === 0}
                                        style={{ marginRight: '4px' }}
                                    >↑</button>
                                    <button
                                        onClick={() => handleMoveCat(cat, 'down')}
                                        disabled={idx === sortedCats.length - 1}
                                        style={{ marginRight: '8px' }}
                                    >↓</button>
                                    <strong style={{ marginRight: '12px' }}>{cat.title}</strong>
                                    <small style={{ color: '#888', marginRight: '12px' }}>
                                        {cat.items?.length ?? 0} питань
                                    </small>
                                    <button onClick={() => navigate(`/projects/${pid}/categories/${cat.id}`)}>Відкрити</button>
                                    {' '}
                                    <button onClick={() => startEditCat(cat)}>✏️</button>
                                    {' '}
                                    <button onClick={() => handleDeleteCat(cat.id)} style={{ color: 'red' }}>🗑</button>
                                </span>
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}
