import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api';

export default function Projects() {
    const [projects, setProjects] = useState([]);
    const [loading, setLoading] = useState(true);
    const navigate = useNavigate();

    useEffect(() => {
        const fetchProjects = async () => {
            try {
                const response = await api.get('/api/projects'); // Захищений роут
                setProjects(response.data);
            } catch (err) {
                console.error('Помилка завантаження проєктів:', err);
                // Якщо помилка (наприклад, 401), інтерцептор сам перекине на /login
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

    if (loading) return <p>Завантаження...</p>;

    return (
        <div style={{ padding: '20px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h1>Мої Проєкти</h1>
                <button onClick={handleLogout}>Вийти</button>
            </div>
            
            <hr />

            {projects.length === 0 ? (
                <p>Проєктів ще немає. Створи перший!</p>
            ) : (
                <ul>
                    {projects.map((proj) => (
                        <li key={proj.id} style={{ marginBottom: '10px' }}>
                            <strong>{proj.title}</strong>
                            <br />
                            <small>Створено: {new Date(proj.created_at).toLocaleDateString()}</small>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}