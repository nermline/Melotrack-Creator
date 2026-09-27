import { useEffect, useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import { AnimatePresence, motion } from 'framer-motion';
import { Maximize, Minimize, Play } from 'lucide-react';
import { useLive } from '../lib/live';
import Stage from '../show/Stage';
import { unlockAudio } from '../show/sound';
import { LiveDot } from '../ui/TopBar';

// The projector view. It only displays: control happens on the remote.
export default function Screen() {
    const { pid } = useParams();
    const { show, offset, status } = useLive(pid, 'screen');
    const [unlocked, setUnlocked] = useState(false);
    const [chrome, setChrome] = useState(true);
    const [fs, setFs] = useState(!!document.fullscreenElement);
    const hideTimer = useRef(null);

    useEffect(() => {
        const poke = () => {
            setChrome(true);
            clearTimeout(hideTimer.current);
            hideTimer.current = setTimeout(() => setChrome(false), 2500);
        };
        poke();
        const onFs = () => setFs(!!document.fullscreenElement);
        const onKey = (e) => e.key === 'f' && toggleFs();
        window.addEventListener('mousemove', poke);
        document.addEventListener('fullscreenchange', onFs);
        window.addEventListener('keydown', onKey);
        return () => {
            window.removeEventListener('mousemove', poke);
            document.removeEventListener('fullscreenchange', onFs);
            window.removeEventListener('keydown', onKey);
            clearTimeout(hideTimer.current);
        };
    }, []);

    // Keep the display awake during the show.
    useEffect(() => {
        let lock = null;
        const acquire = async () => {
            try {
                lock = await navigator.wakeLock?.request('screen');
            } catch {
                /* not supported or denied */
            }
        };
        acquire();
        const onVis = () => document.visibilityState === 'visible' && acquire();
        document.addEventListener('visibilitychange', onVis);
        return () => {
            document.removeEventListener('visibilitychange', onVis);
            lock?.release?.();
        };
    }, []);

    const title = show?.title;
    useEffect(() => {
        document.title = title ? `${title} — екран` : 'Екран';
    }, [title]);

    const start = () => {
        unlockAudio();
        setUnlocked(true);
        if (!document.fullscreenElement) document.documentElement.requestFullscreen?.().catch(() => {});
    };

    return (
        <div className="fixed inset-0 bg-black" style={{ cursor: chrome ? 'default' : 'none' }}>
            <Stage view={show} offset={offset} unlocked={unlocked} />

            <AnimatePresence>
                {!unlocked && (
                    <motion.button
                        type="button"
                        onClick={start}
                        className="absolute inset-0 z-40 flex flex-col items-center justify-center gap-5 bg-black/80 text-ink backdrop-blur-sm"
                        exit={{ opacity: 0 }}
                    >
                        <motion.span
                            className="grid h-28 w-28 place-items-center rounded-full bg-accent text-accent-ink"
                            animate={{ scale: [1, 1.08, 1] }}
                            transition={{ duration: 1.6, repeat: Infinity }}
                        >
                            <Play size={48} fill="currentColor" />
                        </motion.span>
                        <span className="font-display text-2xl font-bold">Увімкнути екран</span>
                        <span className="max-w-md text-center text-sm text-dim">
                            Браузер дозволяє звук лише після натискання. Далі керуйте показом з пульта.
                        </span>
                    </motion.button>
                )}
            </AnimatePresence>

            <AnimatePresence>
                {chrome && unlocked && (
                    <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="absolute top-3 right-3 z-40 flex items-center gap-3 rounded-xl bg-black/60 px-3 py-2 backdrop-blur"
                    >
                        <LiveDot status={status} />
                        <button type="button" className="btn btn-sm" onClick={toggleFs}>
                            {fs ? <Minimize size={14} /> : <Maximize size={14} />}
                            {fs ? 'Вийти з повного екрана' : 'На весь екран'}
                        </button>
                    </motion.div>
                )}
            </AnimatePresence>
            {status !== 'online' && unlocked && (
                <div className="absolute bottom-3 left-3 z-40 rounded-lg bg-black/70 px-3 py-1.5">
                    <LiveDot status={status} />
                </div>
            )}
        </div>
    );
}

function toggleFs() {
    if (document.fullscreenElement) document.exitFullscreen?.();
    else document.documentElement.requestFullscreen?.().catch(() => {});
}
