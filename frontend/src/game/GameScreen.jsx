import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { remainingMs, mediaUrl, answerImg } from './useGame';
import { useTicker } from './sound';

// Розміри масштабуються через container query units (cqmin/cqh) відносно
// контейнера показу — тому однаково гарно і на весь екран, і в маленькому
// превʼю-боксі редактора. Для цього на обгортці увімкнено container-type: size.
const S = {
    wrap: {
        position: 'relative', width: '100%', height: '100%', background: '#05060a',
        color: '#fff', overflow: 'hidden', fontFamily: 'var(--font-sans, system-ui, sans-serif)',
        containerType: 'size',
    },
    layer: {
        position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column',
        alignItems: 'center', justifyContent: 'center', textAlign: 'center', padding: '5cqmin',
    },
    big: { fontSize: 'clamp(28px, 9cqmin, 150px)', fontWeight: 800, lineHeight: 1.05, letterSpacing: '-0.02em' },
    sub: { fontSize: 'clamp(13px, 2.8cqmin, 34px)', color: '#aab2c5' },
    eyebrow: {
        fontSize: 'clamp(12px, 2.6cqmin, 30px)', color: 'var(--color-accent2)', fontWeight: 700,
        letterSpacing: '0.14em', textTransform: 'uppercase',
    },
    chip: {
        background: 'rgba(255,255,255,0.07)', border: '1px solid rgba(255,255,255,0.12)',
        padding: '0.9cqmin 2.4cqmin', borderRadius: '2cqmin', fontSize: 'clamp(12px, 2.4cqmin, 30px)',
        backdropFilter: 'blur(6px)',
    },
    overlay: {
        position: 'absolute', inset: 0, zIndex: 30, cursor: 'pointer', background: 'rgba(5,6,10,0.86)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column', gap: '2cqmin',
    },
};

// ─── Декоративне тло (мʼякі рухомі плями) ──────────────────────────────────
function BgGlow() {
    return (
        <div style={{ position: 'absolute', inset: 0, overflow: 'hidden', pointerEvents: 'none' }}>
            <motion.div
                animate={{ x: ['-10%', '12%', '-10%'], y: ['-8%', '10%', '-8%'] }}
                transition={{ duration: 18, repeat: Infinity, ease: 'easeInOut' }}
                style={{ position: 'absolute', top: '-22%', left: '-12%', width: '72cqmin', height: '72cqmin',
                    borderRadius: '50%', background: 'radial-gradient(circle, rgba(124,131,255,0.32), transparent 70%)',
                    filter: 'blur(6cqmin)' }} />
            <motion.div
                animate={{ x: ['8%', '-10%', '8%'], y: ['6%', '-9%', '6%'] }}
                transition={{ duration: 23, repeat: Infinity, ease: 'easeInOut' }}
                style={{ position: 'absolute', bottom: '-26%', right: '-14%', width: '82cqmin', height: '82cqmin',
                    borderRadius: '50%', background: 'radial-gradient(circle, rgba(34,211,238,0.22), transparent 70%)',
                    filter: 'blur(7cqmin)' }} />
        </div>
    );
}

// ─── Аудіо-візуалізатор (стовпчики еквалайзера) ─────────────────────────────
function Equalizer({ playing }) {
    const bars = [0, 1, 2, 3, 4, 5, 6, 7];
    return (
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: '1.6cqmin', height: '26cqmin' }}>
            {bars.map((i) => (
                <motion.div key={i}
                    animate={playing ? { height: ['28%', '100%', '42%', '82%', '30%'] } : { height: '16%' }}
                    transition={playing
                        ? { duration: 0.8 + (i % 4) * 0.18, repeat: Infinity, ease: 'easeInOut' }
                        : { duration: 0.3 }}
                    style={{ width: '3.4cqmin', borderRadius: '2cqmin',
                        background: 'linear-gradient(180deg, var(--color-accent2), var(--color-accent))',
                        boxShadow: '0 0 3cqmin rgba(124,131,255,0.5)' }} />
            ))}
        </div>
    );
}

// ─── Кільце таймера + число (для відліку та роздумів) ───────────────────────
// Світіння робимо радіальним градієнтом ПОЗАДУ кільця (а не SVG-фільтром
// drop-shadow, який обрізається прямокутною областю фільтра у квадрат).
function TimerView({ sec, frac, color, glow, label }) {
    const R = 44, C = 2 * Math.PI * R;
    return (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '3.5cqmin' }}>
            {label && <div style={{ fontSize: 'clamp(14px, 3.2cqmin, 42px)', color: '#aab2c5', letterSpacing: '0.04em' }}>{label}</div>}
            <div style={{ position: 'relative', width: 'min(60cqmin, 78cqh)', aspectRatio: '1 / 1' }}>
                <div style={{ position: 'absolute', inset: '4%', borderRadius: '50%',
                    background: `radial-gradient(circle, ${glow} 0%, transparent 68%)` }} />
                <svg viewBox="0 0 100 100" style={{ position: 'relative', width: '100%', height: '100%', transform: 'rotate(-90deg)' }}>
                    <circle cx="50" cy="50" r={R} fill="none" stroke="rgba(255,255,255,0.08)" strokeWidth="5" />
                    <circle cx="50" cy="50" r={R} fill="none" stroke={color} strokeWidth="5" strokeLinecap="round"
                        strokeDasharray={C} strokeDashoffset={C * (1 - frac)}
                        style={{ transition: 'stroke-dashoffset 0.2s linear' }} />
                </svg>
                <div style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center' }}>
                    <AnimatePresence mode="popLayout">
                        <motion.div key={sec}
                            initial={{ scale: 0.4, opacity: 0 }} animate={{ scale: 1, opacity: 1 }}
                            exit={{ scale: 1.7, opacity: 0 }} transition={{ duration: 0.4, ease: 'easeOut' }}
                            style={{ fontSize: 'clamp(56px, 32cqmin, 380px)', fontWeight: 900, lineHeight: 1,
                                color: '#fff', fontVariantNumeric: 'tabular-nums' }}>
                            {sec}
                        </motion.div>
                    </AnimatePresence>
                </div>
            </div>
        </div>
    );
}

// Плейсхолдер для відповіді без завантаженого фото.
function PhotoPlaceholder() {
    return (
        <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column',
            alignItems: 'center', justifyContent: 'center', gap: '0.8cqmin', color: '#5b6577' }}>
            <span style={{ fontSize: '6cqmin', lineHeight: 1 }}>🖼</span>
            <span style={{ fontSize: 'clamp(10px, 1.6cqmin, 22px)' }}>нема фото</span>
        </div>
    );
}

function BottomProgress({ frac }) {
    return (
        <div style={{ position: 'absolute', left: 0, right: 0, bottom: 0, height: '1.4cqmin',
            background: 'rgba(255,255,255,0.12)' }}>
            <div style={{ height: '100%', width: `${Math.max(0, Math.min(100, frac * 100))}%`,
                background: 'linear-gradient(90deg, var(--color-accent2), var(--color-accent))',
                transition: 'width 0.15s linear' }} />
        </div>
    );
}

const gridV = { hidden: {}, show: { transition: { staggerChildren: 0.06, delayChildren: 0.1 } } };
const cardV = { hidden: { opacity: 0, y: 24, scale: 0.92 }, show: { opacity: 1, y: 0, scale: 1 } };

export default function GameScreen({ pid, project, cats, state, offsetRef }) {
    const [, force] = useState(0);
    const [unlocked, setUnlocked] = useState(false);
    const videoRef = useRef(null);
    const prevSecRef = useRef(-1);
    const { unlock, tick } = useTicker();

    // Тік ~10/с для оновлення таймерів і синхронізації відео.
    useEffect(() => {
        const id = setInterval(() => force((n) => (n + 1) % 1000000), 100);
        return () => clearInterval(id);
    }, []);

    // Синхронізація відео з серверним таймлайном (перемотка/пауза/доєднання).
    useEffect(() => {
        const v = videoRef.current;
        if (!v || !state) return;
        if (state.phase !== 'playing') { v.pause(); return; }
        const dur = state.phase_duration_ms || 0;
        const target = (dur - remainingMs(state, offsetRef.current)) / 1000;
        if (isFinite(target) && Math.abs(v.currentTime - target) > 0.75) {
            try { v.currentTime = target; } catch { /* ignore */ }
        }
        if (state.paused) v.pause();
        else if (unlocked) v.play().catch(() => {});
    });

    const phase = state?.phase;
    const dur = state?.phase_duration_ms || 0;
    const rem = state ? remainingMs(state, offsetRef.current) : 0;
    const sec = Math.max(0, Math.ceil(rem / 1000));
    const frac = dur > 0 ? Math.max(0, Math.min(1, rem / dur)) : 0;

    // Звук тікання при зміні секунди (відлік і роздуми). Останні 3с — вищий тон.
    useEffect(() => {
        const ticking = phase === 'countdown' || phase === 'thinking';
        if (!unlocked || !ticking) { prevSecRef.current = -1; return; }
        if (sec === prevSecRef.current) return;
        prevSecRef.current = sec;
        if (sec > 0) tick({ freq: sec <= 3 ? 1050 : 780, gain: sec <= 3 ? 0.3 : 0.22, dur: sec <= 3 ? 0.07 : 0.05 });
    }, [sec, phase, unlocked, tick]);

    const cat = state ? cats[state.category_index] : null;
    const item = cat?.items?.[state?.item_index];
    const videoShowing = phase === 'playing' && state?.show_video && state?.has_clip;

    return (
        <div style={S.wrap}>
            {!videoShowing && <BgGlow />}

            {!state && <div style={S.layer}><div style={S.sub}>Підключення…</div></div>}

            {state && (
                <AnimatePresence>
                    <motion.div key={phase}
                        initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
                        transition={{ duration: 0.3 }} style={{ position: 'absolute', inset: 0 }}>
                        {phaseContent()}
                    </motion.div>
                </AnimatePresence>
            )}

            {!unlocked && (
                <div style={S.overlay} onClick={() => { setUnlocked(true); unlock(); }}>
                    <motion.div animate={{ scale: [1, 1.08, 1] }} transition={{ duration: 1.6, repeat: Infinity }}
                        style={{ fontSize: 'clamp(40px, 12cqmin, 160px)' }}>▶</motion.div>
                    <div style={S.sub}>Натисніть, щоб увімкнути показ (дозвіл звуку)</div>
                </div>
            )}
        </div>
    );

    function phaseContent() {
        switch (phase) {
            case 'welcome':
                return (
                    <div style={S.layer}>
                        <motion.div initial={{ opacity: 0, y: 18 }} animate={{ opacity: 1, y: 0 }}
                            transition={{ duration: 0.5 }} style={S.eyebrow}>Музична вікторина</motion.div>
                        <motion.div initial={{ opacity: 0, y: 24 }} animate={{ opacity: 1, y: 0 }}
                            transition={{ duration: 0.5, delay: 0.08 }} style={{ ...S.big, marginTop: '1.6cqmin' }}>
                            {project?.title || 'Готові грати?'}
                        </motion.div>
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '1.4cqmin', justifyContent: 'center',
                            maxWidth: '90%', marginTop: '5cqmin' }}>
                            {cats.map((c, i) => (
                                <motion.span key={c.id} initial={{ opacity: 0, scale: 0.8 }} animate={{ opacity: 1, scale: 1 }}
                                    transition={{ delay: 0.28 + i * 0.06, type: 'spring', stiffness: 300, damping: 22 }}
                                    style={S.chip}>{c.title}</motion.span>
                            ))}
                        </div>
                        <motion.div animate={{ opacity: [0.4, 1, 0.4] }} transition={{ duration: 2.2, repeat: Infinity }}
                            style={{ ...S.sub, position: 'absolute', bottom: '5cqmin' }}>Очікуємо початку показу…</motion.div>
                    </div>
                );

            case 'category_title':
                return (
                    <div style={S.layer}>
                        <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }}
                            style={S.eyebrow}>Категорія {state.category_index + 1} / {cats.length}</motion.div>
                        <motion.div initial={{ opacity: 0, y: 26, scale: 0.96 }} animate={{ opacity: 1, y: 0, scale: 1 }}
                            transition={{ duration: 0.5, delay: 0.1, type: 'spring', stiffness: 220, damping: 22 }}
                            style={{ ...S.big, marginTop: '2cqmin' }}>{cat?.title || ''}</motion.div>
                        <motion.div initial={{ scaleX: 0 }} animate={{ scaleX: 1 }} transition={{ duration: 0.5, delay: 0.3 }}
                            style={{ marginTop: '3cqmin', width: '24cqmin', height: '0.8cqmin', borderRadius: 99,
                                background: 'linear-gradient(90deg, var(--color-accent2), var(--color-accent))' }} />
                    </div>
                );

            case 'countdown':
                return <div style={S.layer}><TimerView sec={sec} frac={frac} color="var(--color-accent2)" glow="rgba(34,211,238,0.40)" /></div>;

            case 'playing':
                return videoShowing ? (
                    <div style={{ position: 'absolute', inset: 0, background: '#000' }}>
                        <video key={`${cat.id}-${item.id}`} ref={videoRef} playsInline
                            style={{ width: '100%', height: '100%', objectFit: 'contain', background: '#000' }}
                            src={mediaUrl(pid, cat.id, item.id)} />
                        <BottomProgress frac={1 - frac} />
                    </div>
                ) : (
                    <div style={S.layer}>
                        {cat && item && state.has_clip && (
                            <video key={`${cat.id}-${item.id}`} ref={videoRef} playsInline
                                style={{ display: 'none' }} src={mediaUrl(pid, cat.id, item.id)} />
                        )}
                        <Equalizer playing={!state.paused} />
                        <div style={{ ...S.sub, marginTop: '4cqmin' }}>
                            {state.paused ? '⏸ Пауза' : 'Звучить мелодія…'}
                        </div>
                        <BottomProgress frac={1 - frac} />
                    </div>
                );

            case 'thinking':
                return <div style={S.layer}><TimerView sec={sec} frac={frac} color="var(--color-warn)" glow="rgba(240,176,58,0.40)" label="Час на роздуми" /></div>;

            case 'await_answers':
                return (
                    <div style={S.layer}>
                        <motion.div initial={{ opacity: 0, scale: 0.9 }} animate={{ opacity: 1, scale: 1 }}
                            transition={{ type: 'spring', stiffness: 240, damping: 18 }} style={S.big}>Час вийшов!</motion.div>
                        <motion.div animate={{ opacity: [0.4, 1, 0.4] }} transition={{ duration: 1.8, repeat: Infinity }}
                            style={{ ...S.sub, marginTop: '2cqmin' }}>Готуємо відповіді…</motion.div>
                    </div>
                );

            case 'answers': {
                const items = cat?.items || [];
                const n = items.length;
                // Колонки за кількістю; розмір картки обмежений І по ширині (колонки),
                // І по висоті (рядки) — тому 1 елемент не розтягується на весь екран,
                // а багато елементів вміщаються разом із заголовком.
                const cols = n <= 1 ? 1 : n <= 4 ? n : n <= 6 ? 3 : 4;
                const rows = Math.max(1, Math.ceil(n / cols));
                const cell = `min(${(86 / cols).toFixed(1)}cqw, ${(62 / rows).toFixed(1)}cqh)`;
                return (
                    <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column',
                        alignItems: 'center', justifyContent: 'center', gap: '3cqmin', padding: '4cqmin' }}>
                        <motion.div initial={{ opacity: 0, y: -14 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }}
                            style={{ fontSize: 'clamp(20px, 5cqmin, 70px)', fontWeight: 800, textAlign: 'center', lineHeight: 1.1 }}>
                            {cat?.title || 'Відповіді'}
                        </motion.div>
                        <motion.div variants={gridV} initial="hidden" animate="show"
                            style={{ display: 'grid', gridTemplateColumns: `repeat(${cols}, ${cell})`,
                                gap: '1.8cqmin', justifyContent: 'center' }}>
                            {items.map((it) => (
                                <motion.div key={it.id} variants={cardV}
                                    style={{ background: 'rgba(255,255,255,0.05)', border: '1px solid rgba(255,255,255,0.10)',
                                        borderRadius: '1.6cqmin', padding: '1cqmin', backdropFilter: 'blur(6px)' }}>
                                    <div style={{ position: 'relative', width: '100%', aspectRatio: '1 / 1',
                                        borderRadius: '1.2cqmin', overflow: 'hidden', background: 'rgba(255,255,255,0.04)' }}>
                                        <PhotoPlaceholder />
                                        {it.answer?.image_path && (
                                            <img src={answerImg(it.answer.image_path)} alt=""
                                                onError={(e) => { e.target.style.display = 'none'; }}
                                                style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' }} />
                                        )}
                                    </div>
                                    <div style={{ marginTop: '0.8cqmin', fontSize: 'clamp(11px, 1.7cqmin, 26px)',
                                        textAlign: 'center', lineHeight: 1.2 }}>{it.answer?.title || ''}</div>
                                </motion.div>
                            ))}
                        </motion.div>
                    </div>
                );
            }

            case 'finished':
                return (
                    <div style={S.layer}>
                        <motion.div animate={{ rotate: [0, 12, -12, 0], scale: [1, 1.12, 1] }}
                            transition={{ duration: 2, repeat: Infinity }}
                            style={{ fontSize: 'clamp(50px, 16cqmin, 220px)' }}>🎉</motion.div>
                        <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }}
                            transition={{ duration: 0.5 }} style={{ ...S.big, marginTop: '2cqmin' }}>Дякуємо за гру!</motion.div>
                    </div>
                );

            default:
                return <div style={S.layer}><div style={S.big}>{phase}</div></div>;
        }
    }
}
