import { useParams, useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import { useGameWS, useProject } from '../game/useGame';
import GameRemote from '../game/GameRemote';
import { Button } from '../ui/kit';

export default function RemoteView() {
    const { pid } = useParams();
    const navigate = useNavigate();
    const { cats } = useProject(pid);
    const { state, offsetRef, send, status } = useGameWS(pid, 'remote');

    return (
        <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.22 }} className="mx-auto px-4 py-5"
            style={{ maxWidth: 560, minHeight: '100vh' }}>
            <div className="flex items-center justify-between gap-2 mb-4">
                <h1 className="m-0 text-xl font-bold" style={{ color: '#fff' }}>🎛 Керування екраном</h1>
                <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--color-muted)' }}>
                    <span title={status}>{status === 'connected' ? '🟢' : status === 'connecting' ? '🟡' : '🔴'}</span>
                    <Button size="sm" variant="ghost" onClick={() => navigate(`/projects/${pid}/play`)}>← вихід</Button>
                </div>
            </div>
            <GameRemote cats={cats} state={state} offsetRef={offsetRef} send={send} />
        </motion.div>
    );
}
