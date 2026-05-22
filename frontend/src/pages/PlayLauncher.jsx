import { useNavigate, useParams } from 'react-router-dom';
import { motion } from 'framer-motion';
import { Glass } from '../ui/kit';

const pageVariants = {
    initial: { opacity: 0, y: 18 },
    animate: { opacity: 1, y: 0, transition: { duration: 0.22, ease: [0.22, 0.61, 0.36, 1] } },
    exit:    { opacity: 0, y: -10, transition: { duration: 0.16, ease: 'easeIn' } },
};

// Вибір режиму показу. Оператор бачить лише Екран/Пульт (різні пристрої),
// редактор/адмін додатково мають комбо-превʼю.
export default function PlayLauncher() {
    const { pid } = useParams();
    const navigate = useNavigate();
    const role = localStorage.getItem('role') || '';
    const isEditor = role === 'admin' || role === 'editor';

    const cards = [
        { to: `/projects/${pid}/screen`, icon: '🖥', title: 'Екран', desc: 'Повноекранний показ на проєкторі/телевізорі. Не може керувати показом.' },
        { to: `/projects/${pid}/remote`, icon: '🎛', title: 'Керування екраном', desc: 'Пульт керування показом з окремого пристрою (телефон/планшет).' },
        ...(isEditor ? [{ to: `/projects/${pid}/present`, icon: '👁', title: 'Превʼю', desc: 'Екран і пульт разом — для перевірки перед показом.', accent: true }] : []),
    ];

    return (
        <motion.div variants={pageVariants} initial="initial" animate="animate" exit="exit">
            <div className="mx-auto max-w-3xl px-4 py-6">
                <p className="text-sm mb-3" style={{ color: 'var(--color-muted)' }}>
                    <a href={`/projects/${pid}`} style={{ cursor: 'pointer' }}
                        onClick={(e) => { e.preventDefault(); navigate(`/projects/${pid}`); }}>← Проєкт</a>
                </p>
                <h1 className="text-2xl sm:text-3xl font-bold m-0" style={{ color: '#fff' }}>Показ проєкту</h1>
                <p className="mt-2 mb-5 text-sm" style={{ color: 'var(--color-muted)', maxWidth: 560 }}>
                    Відкрийте «Екран» на пристрої-проєкторі, а «Пульт» — на окремому пристрої керування.
                    Екран навмисно не може керувати показом (захист від випадкових натискань).
                </p>

                <div className="grid gap-3">
                    {cards.map((c, i) => (
                        <motion.div key={c.to} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}
                            transition={{ delay: 0.05 + i * 0.06 }}
                            whileHover={{ y: -2, transition: { duration: 0.13 } }}
                            onClick={() => navigate(c.to)} style={{ cursor: 'pointer' }}>
                            <Glass className="p-4 flex items-center gap-4"
                                style={c.accent ? { borderColor: 'rgba(124,131,255,0.5)' } : undefined}>
                                <div style={{ fontSize: 34, lineHeight: 1, flexShrink: 0 }}>{c.icon}</div>
                                <div className="min-w-0">
                                    <div className="font-semibold text-lg" style={{ color: '#fff' }}>{c.title}</div>
                                    <div className="text-sm" style={{ color: 'var(--color-muted)' }}>{c.desc}</div>
                                </div>
                                <div className="ml-auto" style={{ color: 'var(--color-muted)', fontSize: 22 }}>→</div>
                            </Glass>
                        </motion.div>
                    ))}
                </div>
            </div>
        </motion.div>
    );
}
