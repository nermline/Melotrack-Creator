import { useParams, useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import { useGameWS, useProject } from '../game/useGame';
import GameScreen from '../game/GameScreen';
import GameRemote from '../game/GameRemote';
import { Button } from '../ui/kit';

// Комбо-режим для редактора/адміна: екран (превʼю) і пульт на одній сторінці.
// Одне зʼєднання роль remote — і керує, і відображає.
export default function PresentView() {
    const { pid } = useParams();
    const navigate = useNavigate();
    const { project, cats } = useProject(pid);
    const { state, offsetRef, send } = useGameWS(pid, 'remote');

    return (
        <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.22 }} className="mx-auto px-4 py-5" style={{ maxWidth: 1400 }}>
            <div className="flex items-center justify-between gap-2 flex-wrap mb-4">
                <div>
                    <h1 className="m-0 text-2xl font-bold" style={{ color: '#fff' }}>Превʼю показу</h1>
                    <p className="m-0 mt-1 text-sm" style={{ color: 'var(--color-muted)' }}>екран + пульт на одному пристрої</p>
                </div>
                <Button variant="ghost" onClick={() => navigate(`/projects/${pid}`)}>← Проєкт</Button>
            </div>

            <div className="grid gap-5 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_360px]">
                <div className="glass" style={{ padding: 8, alignSelf: 'start' }}>
                    <div style={{ width: '100%', aspectRatio: '16 / 9', borderRadius: 12, overflow: 'hidden', background: '#000' }}>
                        <GameScreen pid={pid} project={project} cats={cats} state={state} offsetRef={offsetRef} />
                    </div>
                </div>
                <GameRemote cats={cats} state={state} offsetRef={offsetRef} send={send} />
            </div>
        </motion.div>
    );
}
