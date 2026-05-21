import { useParams, Link } from 'react-router-dom';

// Вибір режиму показу. Оператор бачить лише Екран/Пульт (різні пристрої),
// редактор/адмін додатково мають комбо-превʼю.
export default function PlayLauncher() {
    const { pid } = useParams();
    const role = localStorage.getItem('role') || '';
    const isEditor = role === 'admin' || role === 'editor';

    const card = {
        display: 'block', padding: '16px 20px', margin: '10px 0', maxWidth: 440,
        background: '#2d2d3a', color: '#fff', borderRadius: 8, textDecoration: 'none',
    };

    return (
        <div style={{ padding: 20 }}>
            <p><Link to={`/projects/${pid}`}>← Проєкт</Link></p>
            <h1>Показ проєкту</h1>
            <p style={{ color: '#888', maxWidth: 520 }}>
                Для реального показу відкрийте «Екран» на пристрої-проєкторі, а «Пульт» —
                на окремому пристрої керування. Екран навмисно не може керувати показом (захист).
            </p>

            <Link to={`/projects/${pid}/screen`} style={card}>🖥 Екран (показ)</Link>
            <Link to={`/projects/${pid}/remote`} style={card}>🎛 Пульт (керування)</Link>
            {isEditor && (
                <Link to={`/projects/${pid}/present`} style={{ ...card, background: '#2563eb' }}>
                    👁 Превʼю — екран + пульт разом
                </Link>
            )}
        </div>
    );
}
