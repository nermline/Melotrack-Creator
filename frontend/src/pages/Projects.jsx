import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api';

export default function Projects() {
    const [projects, setProjects] = useState([]);
    const [loading, setLoading] = useState(true);
    
    const [isCreating, setIsCreating] = useState(false);
    const [newTitle, setNewTitle] = useState('');
    const [createError, setCreateError] = useState('');

    const navigate = useNavigate();

    useEffect(() => {
        const fetchProjects = async () => {
            try {
                const response = await api.get('/api/projects');
                setProjects(response.data || []);
            } catch (err) {
                console.error('Помилка завантаження проєктів:', err);
            } finally {
                setLoading(false);
            }
        };

        fetchProjects();
    }, []);

    const handleLogout = () => {
        localStorage.removeItem('token');
        navigate('/login');
    };

    const handleCreateProject = async (e) => {
        e.preventDefault();
        setCreateError('');

        try {
            // Відправляємо тільки title, як ти і просив
            const response = await api.post('/api/projects', {
                title: newTitle
            });

            setProjects([...projects, response.data]);
            
            setNewTitle('');
            setIsCreating(false);
        } catch (err) {
            console.error('Помилка при створенні:', err);
            setCreateError('Не вдалося створити проєкт.');
        }
    };

    if (loading) return <p>Завантаження...</p>;

    return (
        <div style={{ padding: '20px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h1>Мої Проєкти</h1>
                <button onClick={handleLogout}>Вийти</button>
            </div>
            
            <hr />

            <div style={{ marginBottom: '20px' }}>
                {!isCreating ? (
                    <button onClick={() => setIsCreating(true)}>+ Створити новий проєкт</button>
                ) : (
                    <div style={{ border: '1px solid #ccc', padding: '15px', width: '300px' }}>
                        <h3>Новий проєкт</h3>
                        {createError && <p style={{ color: 'red', margin: '5px 0' }}>{createError}</p>}
                        
                        <form onSubmit={handleCreateProject} style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                            <input 
                                type="text" 
                                placeholder="Назва проєкту"
                                value={newTitle} 
                                onChange={(e) => setNewTitle(e.target.value)} 
                                required 
                                autoFocus
                            />

                            <div style={{ display: 'flex', gap: '10px', marginTop: '10px' }}>
                                <button type="submit">Зберегти</button>
                                <button type="button" onClick={() => setIsCreating(false)}>Скасувати</button>
                            </div>
                        </form>
                    </div>
                )}
            </div>

            {projects.length === 0 ? (
                <p>Проєктів ще немає.</p>
            ) : (
                <ul style={{ listStyleType: 'none', padding: 0 }}>
                    {projects.map((proj) => (
                        <li key={proj.id} style={{ 
                            border: '1px solid #eee', 
                            padding: '10px', 
                            marginBottom: '10px',
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center'
                        }}>
                            <div>
                                <h3 style={{ margin: '0 0 5px 0' }}>{proj.title}</h3>
                                <small style={{ color: '#888' }}>
                                    Створено: {new Date(proj.created_at).toLocaleDateString()}
                                </small>
                            </div>
                            
                            <button onClick={() => alert(`Відкриваємо: ${proj.id}`)}>
                                Відкрити
                            </button>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}