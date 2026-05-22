import { useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { useGameWS, useProject } from '../game/useGame';
import GameScreen from '../game/GameScreen';

// Повноекранний показ (роль screen — лише відображення, керувати не може).
export default function ScreenView() {
    const { pid } = useParams();
    const navigate = useNavigate();
    const { project, cats, error } = useProject(pid);
    const { state, offsetRef, status } = useGameWS(pid, 'screen');

    // Панель керування (повноекранний/вихід) сама ховається у бездіяльності.
    const [showBar, setShowBar] = useState(true);
    const [isFs, setIsFs] = useState(false);
    const hideRef = useRef(null);

    useEffect(() => {
        const onMove = () => {
            setShowBar(true);
            clearTimeout(hideRef.current);
            hideRef.current = setTimeout(() => setShowBar(false), 2500);
        };
        onMove();
        window.addEventListener('mousemove', onMove);
        window.addEventListener('touchstart', onMove);
        const onFs = () => setIsFs(!!document.fullscreenElement);
        document.addEventListener('fullscreenchange', onFs);
        return () => {
            window.removeEventListener('mousemove', onMove);
            window.removeEventListener('touchstart', onMove);
            document.removeEventListener('fullscreenchange', onFs);
            clearTimeout(hideRef.current);
        };
    }, []);

    const toggleFs = () => {
        if (!document.fullscreenElement) document.documentElement.requestFullscreen?.().catch(() => {});
        else document.exitFullscreen?.();
    };

    if (error) {
        return <div style={{ color: '#fff', background: '#000', height: '100vh', padding: 20 }}>{error}</div>;
    }

    return (
        <div style={{ position: 'fixed', inset: 0, background: '#000' }}>
            <GameScreen pid={pid} project={project} cats={cats} state={state} offsetRef={offsetRef} />

            <AnimatePresence>
                {showBar && (
                    <motion.div initial={{ opacity: 0, y: -12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -12 }}
                        style={{ position: 'absolute', top: 12, right: 12, zIndex: 40, display: 'flex',
                            alignItems: 'center', gap: 8 }}>
                        <span title={status} style={{ fontSize: 18, lineHeight: 1 }}>
                            {status === 'connected' ? '🟢' : status === 'connecting' ? '🟡' : '🔴'}
                        </span>
                        <button className="btn btn-sm" onClick={toggleFs}>{isFs ? '⤢ Вийти з повноекранного' : '⛶ На весь екран'}</button>
                        <button className="btn btn-sm" onClick={() => navigate(`/projects/${pid}/play`)}>✕</button>
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    );
}
