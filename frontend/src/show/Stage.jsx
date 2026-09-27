import { useEffect, useMemo, useRef } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import confetti from 'canvas-confetti';
import '@fontsource-variable/playfair-display';
import '@fontsource-variable/oswald';
import { remainingMs, useTick } from '../lib/live';
import { fmtPoints, joinUrl, plural, points, songs } from '../lib/format';
import { QR } from '../ui';
import { Backdrop, PlayingVisual, RingTimer } from './visuals';
import Answers from './Answers';
import { cues } from './sound';
import './show.css';

const fade = {
    initial: { opacity: 0 },
    animate: { opacity: 1, transition: { duration: 0.45 } },
    exit: { opacity: 0, transition: { duration: 0.3 } },
};

const rise = (delay = 0) => ({
    initial: { opacity: 0, y: '3cqmin' },
    animate: { opacity: 1, y: 0, transition: { delay, duration: 0.6, ease: [0.2, 0.8, 0.2, 1] } },
});

// Stage renders one show state. It is used fullscreen on the projector, inside
// the rehearsal page and (with preview) as a static theme thumbnail.
export default function Stage({ view, offset = 0, unlocked = false, preview = false }) {
    const theme = view?.theme || 'neon';
    const phase = view?.phase;
    const timed = !!view?.timer?.duration_ms && !view.timer.paused;
    useTick(!preview && timed, 100);

    const rem = view ? remainingMs(view.timer, offset) : 0;
    const dur = view?.timer?.duration_ms || 0;
    const sec = Math.ceil(rem / 1000);
    const frac = dur ? rem / dur : 0;

    useCues(view, sec, unlocked && !preview);

    const item = view?.category?.items?.[view.item_index];
    const videoOn = phase === 'playing' && item?.show_video && !!item?.clip_url;
    const key =
        phase === 'playing' || phase === 'thinking'
            ? `${phase}-${view.cat_index}-${view.item_index}`
            : phase === 'results'
              ? 'results'
              : `${phase}-${view?.cat_index}`;

    return (
        <div className="stage" data-theme={theme}>
            <Backdrop theme={theme} dim={phase !== 'welcome' && phase !== 'category' && phase !== 'finished'} />
            {!preview && view && <ClipLayer view={view} offset={offset} unlocked={unlocked} visible={videoOn} />}

            {!view ? (
                <div className="layer">
                    <div className="sub dots">Підключення</div>
                </div>
            ) : (
                <AnimatePresence mode="wait">
                    <motion.div key={key} {...fade} style={{ position: 'absolute', inset: 0, zIndex: videoOn ? 4 : 2 }}>
                        <Phase view={view} sec={sec} frac={frac} rem={rem} preview={preview} videoOn={videoOn} />
                    </motion.div>
                </AnimatePresence>
            )}

            {view && phase !== 'welcome' && phase !== 'finished' && !videoOn && view.cat_count > 0 && (
                <div className="corner" style={{ top: '3.2cqmin', left: '3.6cqmin' }}>
                    {view.title}
                </div>
            )}
            {view && ['countdown', 'playing', 'thinking'].includes(phase) && !videoOn && (
                <div className="corner" style={{ top: '3.2cqmin', right: '3.6cqmin' }}>
                    {view.category?.title}
                </div>
            )}
            {view?.timer?.paused && (
                <div className="corner" style={{ bottom: '3.4cqmin', right: '3.6cqmin', color: 'var(--a2)' }}>
                    ❚❚ пауза
                </div>
            )}
            {(phase === 'playing' || phase === 'thinking') && dur > 0 && (
                <div className="progress">
                    <div style={{ width: `${(1 - frac) * 100}%` }} />
                </div>
            )}
        </div>
    );
}

function Phase({ view, sec, frac, preview, videoOn }) {
    switch (view.phase) {
        case 'welcome':
            return <Welcome view={view} />;
        case 'category':
            return (
                <div className="layer">
                    <motion.div className="eyebrow-s" {...rise(0)}>
                        Категорія {view.cat_index + 1} / {view.cat_count}
                    </motion.div>
                    <motion.div
                        className="display title-xl"
                        style={{ marginTop: '3cqmin', maxWidth: '90cqw' }}
                        initial={{ opacity: 0, scale: 0.92 }}
                        animate={{ opacity: 1, scale: 1, transition: { delay: 0.15, type: 'spring', stiffness: 120, damping: 16 } }}
                    >
                        {view.category?.title}
                    </motion.div>
                    <motion.div
                        className="accent-bar"
                        style={{ marginTop: '4cqmin' }}
                        initial={{ scaleX: 0 }}
                        animate={{ scaleX: 1, transition: { delay: 0.45, duration: 0.6 } }}
                    />
                    {view.category?.items?.length > 0 && (
                        <motion.div className="sub" style={{ marginTop: '3cqmin' }} {...rise(0.6)}>
                            {songs(view.category.items.length)}
                        </motion.div>
                    )}
                </div>
            );
        case 'countdown':
            return (
                <div className="layer">
                    <div className="eyebrow-s" style={{ marginBottom: '3cqmin' }}>
                        Приготуйтесь слухати
                    </div>
                    <RingTimer fraction={frac} seconds={Math.max(sec, 1)} color="var(--a2)" />
                </div>
            );
        case 'playing':
            return videoOn ? null : (
                <div className="layer">
                    <PlayingVisual theme={view.theme} paused={view.timer.paused || preview} number={view.item_index + 1} />
                    <div className="display title-m" style={{ marginTop: '5cqmin' }}>
                        Пісня {view.item_index + 1} <span style={{ opacity: 0.5 }}>/ {view.item_count}</span>
                    </div>
                </div>
            );
        case 'thinking':
            return (
                <div className="layer">
                    <div className="display title-m" style={{ marginBottom: '3.5cqmin' }}>
                        Записуйте відповідь
                    </div>
                    <RingTimer fraction={frac} seconds={Math.max(sec, 0)} color={sec <= 3 ? 'var(--a3)' : 'var(--a1)'} size="46cqmin" />
                    <div className="sub" style={{ marginTop: '3cqmin' }}>
                        Пісня {view.item_index + 1} з {view.item_count}
                    </div>
                </div>
            );
        case 'collect':
            return (
                <div className="layer">
                    <motion.div className="eyebrow-s" {...rise(0)}>
                        {view.category?.title}
                    </motion.div>
                    <motion.div className="display title-l" style={{ marginTop: '2.5cqmin' }} {...rise(0.1)}>
                        Здаємо бланки!
                    </motion.div>
                    <motion.div className="sub dots" style={{ marginTop: '3cqmin' }} {...rise(0.3)}>
                        Готуємо правильні відповіді
                    </motion.div>
                    <Sheets />
                </div>
            );
        case 'answers':
            return <Answers view={view} />;
        case 'scoring':
            return (
                <div className="layer">
                    <PlayingVisual theme={view.theme} paused={preview} number="?" />
                    <div className="display title-l" style={{ marginTop: '5cqmin' }}>
                        Підраховуємо бали
                    </div>
                    <div className="sub dots" style={{ marginTop: '2.5cqmin' }}>
                        Переможців оголосимо за мить
                    </div>
                </div>
            );
        case 'results':
            return <Results view={view} preview={preview} />;
        case 'finished':
            return <Finished view={view} preview={preview} />;
        default:
            return null;
    }
}

function Welcome({ view }) {
    const teams = view.teams || [];
    const join = view.join_code;
    return (
        <div className="layer" style={{ flexDirection: 'row', gap: '6cqw', alignItems: 'center' }}>
            <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', alignItems: join ? 'flex-start' : 'center', textAlign: join ? 'left' : 'center' }}>
                <motion.div className="eyebrow-s" {...rise(0)}>
                    Музичний квіз
                </motion.div>
                <motion.div className="display title-xl" style={{ marginTop: '2cqmin' }} {...rise(0.1)}>
                    {view.title}
                </motion.div>
                {view.categories?.length > 0 && (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '1.2cqmin', marginTop: '4cqmin', justifyContent: join ? 'flex-start' : 'center' }}>
                        {view.categories.map((c, i) => (
                            <motion.span key={c + i} className="pill" {...rise(0.3 + i * 0.06)}>
                                {c}
                            </motion.span>
                        ))}
                    </div>
                )}
                {teams.length > 0 && (
                    <div style={{ marginTop: '5cqmin', width: '100%' }}>
                        <div className="sub" style={{ marginBottom: '1.5cqmin' }}>
                            {teams.length} {plural(teams.length, 'команда', 'команди', 'команд')} вже з нами
                        </div>
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '1cqmin', justifyContent: join ? 'flex-start' : 'center', maxHeight: '22cqh', overflow: 'hidden' }}>
                            <AnimatePresence>
                                {teams.map((t) => (
                                    <motion.span
                                        key={t}
                                        layout
                                        className="pill"
                                        style={{ color: 'var(--a2)' }}
                                        initial={{ opacity: 0, scale: 0.5 }}
                                        animate={{ opacity: 1, scale: 1 }}
                                        transition={{ type: 'spring', stiffness: 300, damping: 18 }}
                                    >
                                        {t}
                                    </motion.span>
                                ))}
                            </AnimatePresence>
                        </div>
                    </div>
                )}
            </div>
            {join && (
                <motion.div
                    className="card"
                    style={{ padding: '3cqmin', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '2cqmin', flexShrink: 0 }}
                    {...rise(0.4)}
                >
                    <div className="display" style={{ fontSize: '4.4cqmin' }}>
                        Реєстрація команд
                    </div>
                    <div style={{ width: '34cqmin', height: '34cqmin' }}>
                        <QR value={joinUrl(join)} size="100%" dark="#0b0b12" />
                    </div>
                    <div className="sub" style={{ fontFamily: 'ui-monospace, monospace', fontSize: '3cqmin' }}>
                        {joinUrl(join).replace(/^https?:\/\//, '')}
                    </div>
                </motion.div>
            )}
            <motion.div
                className="sub"
                style={{ position: 'absolute', bottom: '4cqmin', left: 0, right: 0, textAlign: 'center' }}
                animate={{ opacity: [0.35, 1, 0.35] }}
                transition={{ duration: 2.4, repeat: Infinity }}
            >
                Скоро почнемо
            </motion.div>
        </div>
    );
}

function Sheets() {
    return (
        <div style={{ position: 'relative', width: '22cqmin', height: '16cqmin', marginTop: '6cqmin' }}>
            {[0, 1, 2].map((i) => (
                <motion.div
                    key={i}
                    className="card"
                    style={{ position: 'absolute', inset: 0, background: 'var(--ink)', opacity: 0.9 - i * 0.2, borderRadius: '1cqmin' }}
                    animate={{ rotate: [-8 + i * 8, -4 + i * 8, -8 + i * 8], y: [0, -i * 6, 0] }}
                    transition={{ duration: 2.2, repeat: Infinity, delay: i * 0.2 }}
                >
                    {[0, 1, 2, 3].map((l) => (
                        <div key={l} style={{ height: '0.8cqmin', margin: '2cqmin 2cqmin 0', width: `${60 + ((l * 13) % 30)}%`, background: 'var(--bg)', opacity: 0.25, borderRadius: 4 }} />
                    ))}
                </motion.div>
            ))}
        </div>
    );
}

function useConfetti(fire, preview, theme) {
    const canvasRef = useRef(null);
    useEffect(() => {
        if (!fire || preview || !canvasRef.current) return;
        const shoot = confetti.create(canvasRef.current, { resize: true, useWorker: true });
        const colors = { neon: ['#ff2bd6', '#22e4ff', '#7b5cff', '#ffffff'], vinyl: ['#f28c28', '#e7bb45', '#f6ead2', '#b5462b'], stage: ['#f5c542', '#fff1b8', '#ffffff'] }[theme];
        const end = Date.now() + 2500;
        let raf;
        const frame = () => {
            shoot({ particleCount: 5, angle: 60, spread: 70, origin: { x: 0, y: 0.7 }, colors });
            shoot({ particleCount: 5, angle: 120, spread: 70, origin: { x: 1, y: 0.7 }, colors });
            if (Date.now() < end) raf = requestAnimationFrame(frame);
        };
        shoot({ particleCount: 160, spread: 100, origin: { y: 0.45 }, colors });
        frame();
        return () => {
            cancelAnimationFrame(raf);
            shoot.reset();
        };
    }, [fire, preview, theme]);
    return <canvas ref={canvasRef} style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 8 }} />;
}

const MEDAL = { 1: 'var(--a2)', 2: '#d9dde6', 3: '#e3a36b' };

// podiumFont shrinks a team name until its longest word fits the pedestal,
// so names wrap between words and never inside one.
function podiumFont(name, base, width) {
    const longest = Math.max(...name.split(/\s+/).map((w) => w.length), 1);
    return Math.min(base, (width * 0.92) / (longest * 0.82)).toFixed(2);
}

function Results({ view, preview }) {
    const board = view.board || [];
    const n = board.length;
    const shown = (i) => i >= n - view.revealed;
    const podium = board.slice(0, 3);
    const rest = board.slice(3);
    const allOut = view.revealed >= n && n > 0;
    const canvas = useConfetti(allOut, preview, view.theme);

    const order = [1, 0, 2].filter((i) => i < podium.length);
    const heights = { 0: 34, 1: 25, 2: 18 };

    return (
        <div className="layer" style={{ justifyContent: 'flex-start', paddingTop: '5cqmin' }}>
            {canvas}
            <motion.div className="display title-l" {...rise(0)}>
                {allOut ? 'Переможці!' : 'Результати'}
            </motion.div>
            <div style={{ flex: 1, width: '100%', display: 'flex', gap: '4cqw', alignItems: 'flex-end', justifyContent: 'center', marginTop: '2cqmin' }}>
                {rest.length > 0 && (
                    <div
                        style={{
                            flex: '0 1 40cqw',
                            alignSelf: 'center',
                            display: 'grid',
                            gridTemplateColumns: rest.length > 8 ? '1fr 1fr' : '1fr',
                            gap: '0.8cqmin 2cqmin',
                            fontSize: `min(3.8cqmin, ${(52 / Math.ceil(rest.length / (rest.length > 8 ? 2 : 1))).toFixed(1)}cqh)`,
                        }}
                    >
                        {rest.map((b, j) => {
                            const i = j + 3;
                            return (
                                <motion.div
                                    key={b.team_id}
                                    className="card"
                                    style={{ display: 'flex', alignItems: 'center', gap: '1.5cqmin', padding: '0.7em 1em', textAlign: 'left' }}
                                    animate={{ opacity: shown(i) ? 1 : 0.18, x: shown(i) ? 0 : '-2cqmin' }}
                                    transition={{ delay: shown(i) ? (n - 1 - i) * 0.08 : 0 }}
                                >
                                    <span className="display" style={{ color: 'var(--dim)', fontStyle: 'normal', minWidth: '1.6em' }}>
                                        {b.rank}
                                    </span>
                                    <span style={{ flex: 1, fontWeight: 800, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                        {shown(i) ? b.name : '• • •'}
                                    </span>
                                    <span className="display" style={{ fontStyle: 'normal' }}>
                                        {shown(i) ? fmtPoints(b.points) : ''}
                                    </span>
                                </motion.div>
                            );
                        })}
                    </div>
                )}
                <div style={{ display: 'flex', alignItems: 'flex-end', gap: '1.6cqmin', height: '100%' }}>
                    {order.map((i) => {
                        const b = podium[i];
                        const on = shown(i);
                        const color = MEDAL[b.rank] || 'var(--dim)';
                        return (
                            <div key={b.team_id} style={{ width: i === 0 ? '28cqmin' : '24cqmin', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'flex-end', height: '100%' }}>
                                <AnimatePresence>
                                    {on && (
                                        <motion.div
                                            style={{ textAlign: 'center', marginBottom: '1.5cqmin', width: '100%' }}
                                            initial={{ opacity: 0, y: '6cqmin', scale: 0.7 }}
                                            animate={{ opacity: 1, y: 0, scale: 1 }}
                                            transition={{ type: 'spring', stiffness: 140, damping: 14 }}
                                        >
                                            <div
                                                className="display"
                                                style={{
                                                    fontSize: `${podiumFont(b.name, i === 0 ? 5.2 : 4, i === 0 ? 28 : 24)}cqmin`,
                                                    lineHeight: 1.1,
                                                    textShadow: 'var(--glow)',
                                                    overflowWrap: 'break-word',
                                                }}
                                            >
                                                {b.name}
                                            </div>
                                            <div className="sub" style={{ marginTop: '0.6cqmin' }}>
                                                {points(b.points)}
                                            </div>
                                        </motion.div>
                                    )}
                                </AnimatePresence>
                                <motion.div
                                    style={{
                                        width: '100%',
                                        borderRadius: '1.5cqmin 1.5cqmin 0 0',
                                        background: `linear-gradient(180deg, color-mix(in srgb, ${color} 55%, transparent), color-mix(in srgb, ${color} 12%, transparent))`,
                                        border: `1px solid color-mix(in srgb, ${color} 60%, transparent)`,
                                        borderBottom: 'none',
                                        display: 'grid',
                                        placeItems: 'start center',
                                        paddingTop: '1.5cqmin',
                                    }}
                                    initial={{ height: 0 }}
                                    animate={{ height: `${heights[i]}cqh` }}
                                    transition={{ duration: 0.8, delay: 0.2 + i * 0.1 }}
                                >
                                    <span className="display" style={{ fontSize: '7cqmin', color, fontStyle: 'normal', opacity: on ? 1 : 0.5 }}>
                                        {on ? b.rank : '?'}
                                    </span>
                                </motion.div>
                            </div>
                        );
                    })}
                </div>
            </div>
        </div>
    );
}

function Finished({ view, preview }) {
    const winners = (view.board || []).filter((b) => b.rank === 1);
    const canvas = useConfetti(winners.length > 0, preview, view.theme);
    return (
        <div className="layer">
            {canvas}
            <motion.div className="display title-xl" {...rise(0)}>
                Дякуємо за гру!
            </motion.div>
            {winners.length > 0 && (
                <motion.div className="sub" style={{ marginTop: '4cqmin', fontSize: '5cqmin' }} {...rise(0.3)}>
                    Переможці:{' '}
                    <b style={{ color: 'var(--a2)' }}>{winners.map((w) => w.name).join(', ')}</b>
                </motion.div>
            )}
            <motion.div className="accent-bar" style={{ marginTop: '4cqmin' }} initial={{ scaleX: 0 }} animate={{ scaleX: 1, transition: { delay: 0.5 } }} />
        </div>
    );
}

// ClipLayer keeps every clip of the current category loaded and plays the
// active one in sync with the server timer. The <video> elements stay mounted
// across phases so the next song starts instantly.
function ClipLayer({ view, offset, unlocked, visible }) {
    const refs = useRef({});
    const items = useMemo(() => (view.category?.items || []).filter((i) => i.clip_url), [view.category]);
    const active = view.phase === 'playing' ? view.category?.items?.[view.item_index] : null;

    useEffect(() => {
        const sync = () => {
            for (const it of items) {
                const v = refs.current[it.id];
                if (!v) continue;
                if (active && it.id === active.id && unlocked) {
                    const elapsed = (view.timer.duration_ms - remainingMs(view.timer, offset)) / 1000;
                    if (view.timer.paused) {
                        if (!v.paused) v.pause();
                        if (Math.abs(v.currentTime - elapsed) > 0.15) v.currentTime = elapsed;
                    } else {
                        if (Math.abs(v.currentTime - elapsed) > 0.35) v.currentTime = Math.max(elapsed, 0);
                        if (v.paused && elapsed < view.timer.duration_ms / 1000 - 0.1) v.play().catch(() => {});
                    }
                } else if (!v.paused) {
                    v.pause();
                } else if (v.currentTime !== 0 && (!active || it.id !== active.id)) {
                    v.currentTime = 0;
                }
            }
        };
        sync();
        const id = setInterval(sync, 250);
        return () => clearInterval(id);
    }, [items, active, view.timer, offset, unlocked]);

    return (
        <div className={`clips ${visible ? 'visible' : ''}`}>
            {items.map((it) => (
                <video
                    key={it.id}
                    ref={(el) => {
                        if (el) refs.current[it.id] = el;
                        else delete refs.current[it.id];
                    }}
                    src={it.clip_url}
                    preload="auto"
                    playsInline
                    className={active?.id === it.id && visible ? 'active' : ''}
                />
            ))}
        </div>
    );
}

// Sound cues follow the timer: ticks in the countdown and at the end of the
// thinking pause, a chord when answers appear, a fanfare for the winner.
function useCues(view, sec, enabled) {
    const last = useRef({ phase: null, sec: null, revealed: 0 });
    useEffect(() => {
        if (!view) return;
        const prev = last.current;
        const phaseChanged = prev.phase !== view.phase;
        if (enabled && !view.timer?.paused) {
            if (view.phase === 'countdown' && sec !== prev.sec && sec > 0) cues.tick(false);
            if (view.phase === 'playing' && phaseChanged && prev.phase === 'countdown') cues.go();
            if (view.phase === 'thinking' && sec !== prev.sec && sec > 0 && sec <= 5) cues.tick(sec <= 3);
            if (view.phase === 'answers' && phaseChanged) cues.reveal();
            if (view.phase === 'results' && view.revealed > prev.revealed) {
                if (view.revealed >= (view.board?.length || 0)) cues.fanfare();
                else cues.reveal();
            }
        }
        last.current = { phase: view.phase, sec, revealed: view.revealed };
    }, [view, sec, enabled]);
}
