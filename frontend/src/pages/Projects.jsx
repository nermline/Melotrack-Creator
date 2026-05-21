import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api';

export default function Projects() {
    const [projects, setProjects] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');

    // Create
    const [isCreating, setIsCreating] = useState(false);
    const [newTitle, setNewTitle] = useState('');
    const [createError, setCreateError] = useState('');

    // Edit
    const [editingId, setEditingId] = useState(null);
    const [editTitle, setEditTitle] = useState('');
    const [editError, setEditError] = useState('');

    const navigate = useNavigate();

    useEffect(() => {
        api.get('/api/projects')
            .then(r => setProjects(r.data || []))
            .catch(() => setError('Помилка завантаження проєктів'))
            .finally(() => setLoading(false));
    }, []);

    const handleLogout = async () => {
        try { await api.post('/api/logout'); } catch {}
        localStorage.removeItem('token');
        navigate('/login');
    };

    const handleCreate = async (e) => {
        e.preventDefault();
        setCreateError('');
        try {
            const r = await api.post('/api/projects', { title: newTitle });
            setProjects(prev => [...prev, r.data]);
            setNewTitle('');
            setIsCreating(false);
        } catch (err) {
            const msg = err.response?.data?.error;
            setCreateError(msg === 'title is already taken' ? 'Назва вже зайнята' : 'Не вдалося створити');
        }
    };

    const startEdit = (proj) => {
        setEditingId(proj.id);
        setEditTitle(proj.title);
        setEditError('');
    };

    const handleEdit = async (e, id) => {
        e.preventDefault();
        setEditError('');
        try {
            const r = await api.put(`/api/projects/${id}`, { title: editTitle });
            setProjects(prev => prev.map(p => p.id === id ? { ...p, title: r.data.title } : p));
            setEditingId(null);
        } catch (err) {
            const msg = err.response?.data?.error;
            setEditError(msg === 'title is already taken' ? 'Назва вже зайнята' : 'Не вдалося оновити');
        }
    };

    const handleDelete = async (id) => {
        if (!confirm('Видалити проєкт разом з усіма категоріями та медіа?')) return;
        try {
            await api.delete(`/api/projects/${id}`);
            setProjects(prev => prev.filter(p => p.id !== id));
        } catch {
            alert('Не вдалося видалити проєкт');
        }
    };

    if (loading) return <p>Завантаження...</p>;

    return (
        <div style={{ padding: '16px', maxWidth: '700px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h1>Melotrack — Проєкти</h1>
                <button onClick={handleLogout}>Вийти</button>
            </div>

            {error && <p style={{ color: 'red' }}>{error}</p>}

            <hr />

            {/* Create */}
            {!isCreating ? (
                <button onClick={() => { setIsCreating(true); setCreateError(''); }}>+ Новий проєкт</button>
            ) : (
                <form onSubmit={handleCreate} style={{ marginBottom: '12px' }}>
                    <input
                        autoFocus
                        value={newTitle}
                        onChange={e => setNewTitle(e.target.value)}
                        placeholder="Назва проєкту"
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

            {projects.length === 0 ? (
                <p>Проєктів ще немає.</p>
            ) : (
                <ul style={{ listStyle: 'none', padding: 0 }}>
                    {projects.map(proj => (
                        <li key={proj.id} style={{ borderBottom: '1px solid #ccc', padding: '8px 0' }}>
                            {editingId === proj.id ? (
                                <form onSubmit={e => handleEdit(e, proj.id)} style={{ display: 'inline' }}>
                                    <input
                                        autoFocus
                                        value={editTitle}
                                        onChange={e => setEditTitle(e.target.value)}
                                        required
                                    />
                                    {' '}
                                    <button type="submit">OK</button>
                                    {' '}
                                    <button type="button" onClick={() => setEditingId(null)}>✕</button>
                                    {editError && <span style={{ color: 'red', marginLeft: '8px' }}>{editError}</span>}
                                </form>
                            ) : (
                                <span>
                                    <strong style={{ marginRight: '12px' }}>{proj.title}</strong>
                                    <small style={{ color: '#888', marginRight: '12px' }}>
                                        {new Date(proj.created_at).toLocaleDateString()}
                                    </small>
                                    <button onClick={() => navigate(`/projects/${proj.id}`)}>Відкрити</button>
                                    {' '}
                                    <button onClick={() => startEdit(proj)}>Перейменувати</button>
                                    {' '}
                                    <button onClick={() => handleDelete(proj.id)} style={{ color: 'red' }}>Видалити</button>
                                </span>
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}
