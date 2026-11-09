import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import confetti from 'canvas-confetti';
import '@fontsource-variable/playfair-display';
import '@fontsource-variable/oswald';
import { remainingMs, useTick } from '../lib/live';
import { fmtPoints, plural, points, songs } from '../lib/format';
import { Backdrop, PlayingVisual, RingTimer } from './visuals';
import Answers from './Answers';
import { cues, routeMedia, setVolume } from './sound';
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
// the rehearsal page and (with preview) as a static theme thumbnail. It keeps a
// 16:9 shape and everything inside is sized in container units, so all of them
// show the same picture, only bigger or smaller. (It is not a full-size stage
// scaled with a transform: Safari on iPhone would keep full-size layers in
// memory for that.)
export default function Stage(props) {
    const boxRef = useRef(null);
    const [size, setSize] = useState({ w: 0, h: 0 });
    useLayoutEffect(() => {
        const box = boxRef.current;
        const fit = () => {
            const unit = Math.min(box.clientWidth / 16, box.clientHeight / 9);
            setSize({ w: Math.floor(unit * 16), h: Math.floor(unit * 9) });
        };
        fit();
        const ro = new ResizeObserver(fit);
        ro.observe(box);
        return () => ro.disconnect();
    }, []);
    return (
        <div ref={boxRef} style={{ position: 'absolute', inset: 0, overflow: 'hidden', display: 'grid', placeItems: 'center' }}>
            <div style={{ position: 'relative', width: size.w, height: size.h, visibility: size.w ? 'visible' : 'hidden' }}>
                <StageView {...props} />
            </div>
        </div>
    );
}

function StageView({ view, offset = 0, unlocked = false, preview = false }) {
    const theme = view?.theme || 'neon';
    const phase = view?.phase;
    const timed = !!view?.timer?.duration_ms && !view.timer.paused;
    useTick(!preview && timed, 100);

    const rem = view ? remainingMs(view.timer, offset) : 0;
    const dur = view?.timer?.duration_ms || 0;
    const sec = Math.ceil(rem / 1000);
    const frac = dur ? rem / dur : 0;

    useCues(view, sec, unlocked && !preview);
    const volume = view?.volume ?? 1;
    useEffect(() => {
        if (!preview) setVolume(volume);
    }, [volume, preview]);

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
        case 'leaderboard':
            return <Leaderboard view={view} />;
        case 'finished':
            return <Finished view={view} preview={preview} />;
        default:
            return null;
    }
}

// useShrinkToFit sets --k on the box's content to the largest scale (up to 1)
// at which the content fits the box's height.
function useShrinkToFit(ref, key) {
    useLayoutEffect(() => {
        const box = ref.current;
        if (!box) return;
        const inner = box.firstElementChild;
        let alive = true;
        let raf = 0;
        const fits = (k) => {
            inner.style.setProperty('--k', String(k));
            return inner.offsetHeight <= box.clientHeight;
        };
        const fit = () => {
            if (!alive || fits(1)) return;
            let lo = 0.3;
            let hi = 1;
            for (let i = 0; i < 8; i++) {
                const mid = (lo + hi) / 2;
                if (fits(mid)) lo = mid;
                else hi = mid;
            }
            fits(lo);
        };
        fit();
        // Web fonts arrive late and change text widths; the stage also resizes.
        document.fonts?.ready.then(fit);
        const ro = new ResizeObserver(() => {
            cancelAnimationFrame(raf);
            raf = requestAnimationFrame(fit);
        });
        ro.observe(box);
        return () => {
            alive = false;
            ro.disconnect();
            cancelAnimationFrame(raf);
        };
    }, [ref, key]);
}

// k scales a size in cqmin by the fit factor of the welcome slide.
const k = (n) => `calc(${n}cqmin * var(--k, 1))`;

// Welcome shrinks its content until everything fits above "Скоро почнемо":
// a show can have many categories and teams.
function Welcome({ view }) {
    const teams = view.teams || [];
    const cats = view.categories || [];
    const boxRef = useRef(null);
    useShrinkToFit(boxRef, [view.title, ...cats, '', ...teams].join('\n'));
    const pill = { fontSize: k(3.3), padding: `${k(1)} ${k(2.6)}` };
    return (
        <div className="layer" style={{ paddingBottom: '3.5cqmin' }}>
            <div ref={boxRef} style={{ flex: 1, minHeight: 0, width: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <motion.div className="eyebrow-s" style={{ fontSize: k(3.3) }} {...rise(0)}>
                        Музичний квіз
                    </motion.div>
                    <motion.div
                        className="display title-xl"
                        // The title shrinks less than the lists below it.
                        style={{ marginTop: k(2), fontSize: 'calc(11cqmin * max(var(--k, 1), 0.7))' }}
                        {...rise(0.1)}
                    >
                        {view.title}
                    </motion.div>
                    {cats.length > 0 && (
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: k(1.2), marginTop: k(4), justifyContent: 'center' }}>
                            {cats.map((c, i) => (
                                <motion.span key={c + i} className="pill" style={pill} {...rise(0.3 + i * 0.04)}>
                                    {c}
                                </motion.span>
                            ))}
                        </div>
                    )}
                    {teams.length > 0 && (
                        <div style={{ marginTop: k(5), width: '100%' }}>
                            <div className="sub" style={{ fontSize: k(4), marginBottom: k(1.5) }}>
                                {teams.length} {plural(teams.length, 'команда', 'команди', 'команд')} вже з нами
                            </div>
                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: k(1), justifyContent: 'center' }}>
                                <AnimatePresence>
                                    {teams.map((t) => (
                                        <motion.span
                                            key={t}
                                            layout
                                            className="pill"
                                            style={{ ...pill, color: 'var(--a2)' }}
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
            </div>
            <motion.div
                className="sub"
                style={{ marginTop: '2cqmin', flexShrink: 0 }}
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
                        <div key={l} style={{ height: '0.8cqmin', margin: '2cqmin 2cqmin 0', width: `${60 + ((l * 13) % 30)}%`, background: 'var(--bg)', opacity: 0.25, borderRadius: '0.4cqmin' }} />
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

// podiumPlaces groups the first three positions by rank: teams with equal
// points share a pedestal (the server counts the same places).
function podiumPlaces(board) {
    const places = [];
    board.forEach((b, i) => {
        if (i > 0 && b.rank === board[i - 1].rank) places[places.length - 1].push(b);
        else if (i < 3) places.push([b]);
    });
    return places;
}

const PEDESTAL = [
    { width: 38, height: 36 },
    { width: 31, height: 27 },
    { width: 31, height: 20 },
];

// Results: only the podium, revealed from the lowest place up.
function Results({ view, preview }) {
    const places = podiumPlaces(view.board || []);
    const n = places.length;
    const shown = (p) => p >= n - view.revealed;
    const allOut = n > 0 && view.revealed >= n;
    const canvas = useConfetti(allOut, preview, view.theme);
    const order = [1, 0, 2].filter((p) => p < n);

    return (
        <div className="layer" style={{ justifyContent: 'flex-start', paddingTop: '5cqmin' }}>
            {canvas}
            <motion.div className="display title-l" {...rise(0)}>
                {allOut ? 'Переможці!' : 'Хто ж переміг?'}
            </motion.div>
            <div style={{ flex: 1, display: 'flex', alignItems: 'flex-end', justifyContent: 'center', gap: '2cqmin', marginTop: '2cqmin' }}>
                {order.map((p) => {
                    const teams = places[p];
                    const on = shown(p);
                    const rank = teams[0].rank;
                    const color = MEDAL[rank] || 'var(--dim)';
                    const { width, height } = PEDESTAL[p];
                    // Tied teams share the pedestal: smaller names, one per line.
                    const base = (p === 0 ? 5.6 : 4.4) / Math.sqrt(teams.length);
                    return (
                        <div
                            key={rank}
                            style={{ width: `${width}cqmin`, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'flex-end', height: '100%' }}
                        >
                            <AnimatePresence>
                                {on && (
                                    <motion.div
                                        style={{ textAlign: 'center', marginBottom: '1.5cqmin', width: '100%' }}
                                        initial={{ opacity: 0, y: '6cqmin', scale: 0.7 }}
                                        animate={{ opacity: 1, y: 0, scale: 1 }}
                                        transition={{ type: 'spring', stiffness: 140, damping: 14 }}
                                    >
                                        {teams.map((b) => (
                                            <div
                                                key={b.team_id}
                                                className="display"
                                                style={{
                                                    fontSize: `${podiumFont(b.name, base, width)}cqmin`,
                                                    lineHeight: 1.1,
                                                    textShadow: 'var(--glow)',
                                                    overflowWrap: 'break-word',
                                                    marginTop: '0.4cqmin',
                                                }}
                                            >
                                                {b.name}
                                            </div>
                                        ))}
                                        <div className="sub" style={{ marginTop: '0.8cqmin' }}>
                                            {points(teams[0].points)}
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
                                animate={{ height: `${height}cqh` }}
                                transition={{ duration: 0.8, delay: 0.2 + p * 0.1 }}
                            >
                                <span className="display" style={{ fontSize: '8cqmin', color, fontStyle: 'normal', opacity: on ? 1 : 0.5 }}>
                                    {on ? rank : '?'}
                                </span>
                            </motion.div>
                        </div>
                    );
                })}
            </div>
        </div>
    );
}

// Leaderboard: every team in one column, best first (two columns when there
// are too many to read in one).
function Leaderboard({ view }) {
    const board = view.board || [];
    const n = board.length;
    const cols = n > 14 ? 2 : 1;
    const rows = Math.ceil(n / cols);
    const row = Math.min(8.5, 70 / Math.max(rows, 1));
    return (
        <div className="layer" style={{ justifyContent: 'flex-start', paddingTop: '5cqmin' }}>
            <motion.div className="display title-l" {...rise(0)}>
                Підсумкова таблиця
            </motion.div>
            <div
                style={{
                    flex: 1,
                    display: 'grid',
                    gridTemplateColumns: `repeat(${cols}, ${cols === 1 ? '66cqw' : '42cqw'})`,
                    gridTemplateRows: `repeat(${rows}, ${row.toFixed(2)}cqh)`,
                    gridAutoFlow: 'column',
                    gap: `${(row * 0.14).toFixed(2)}cqh 3cqw`,
                    alignContent: 'center',
                    justifyContent: 'center',
                    marginTop: '2cqmin',
                    fontSize: `${(row * 0.42).toFixed(2)}cqh`,
                }}
            >
                {board.map((b, i) => {
                    const medal = MEDAL[b.rank];
                    return (
                        <motion.div
                            key={b.team_id}
                            className="card"
                            style={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: '1em',
                                padding: '0 1em',
                                textAlign: 'left',
                                // Solid enough to read over the theme's backdrop on a projector.
                                background: 'color-mix(in srgb, var(--bg) 78%, transparent)',
                                borderColor: medal ? `color-mix(in srgb, ${medal} 55%, transparent)` : undefined,
                            }}
                            initial={{ opacity: 0, x: '-3cqmin' }}
                            animate={{ opacity: 1, x: 0, transition: { delay: 0.15 + i * 0.05 } }}
                        >
                            <span className="display" style={{ minWidth: '1.8em', fontStyle: 'normal', color: medal || 'var(--dim)' }}>
                                {b.rank}
                            </span>
                            <span style={{ flex: 1, minWidth: 0, fontWeight: 800, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                {b.name}
                            </span>
                            <span className="display" style={{ fontStyle: 'normal', color: medal || 'var(--ink)' }}>
                                {fmtPoints(b.points)}
                            </span>
                        </motion.div>
                    );
                })}
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

// ClipLayer plays the active clip in sync with the server timer. The next clip
// is already loaded and its <video> stays mounted, so it starts instantly.
function ClipLayer({ view, offset, unlocked, visible }) {
    const refs = useRef({});
    // Only the current clip and the next one are loaded: a phone (Safari on
    // iPhone above all) runs out of memory with a player for every song.
    const songPhase = ['category', 'countdown', 'playing', 'thinking'].includes(view.phase);
    const index = view.item_index;
    const items = useMemo(
        () => (songPhase ? (view.category?.items || []).slice(index, index + 2).filter((i) => i.clip_url) : []),
        [view.category, index, songPhase],
    );
    const active = view.phase === 'playing' ? view.category?.items?.[view.item_index] : null;

    // Once sound is unlocked, clips play through the show's volume.
    useEffect(() => {
        if (unlocked) items.forEach((it) => routeMedia(refs.current[it.id]));
    }, [items, unlocked]);

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
                if (view.revealed >= (view.podium || 0)) cues.fanfare();
                else cues.reveal();
            }
            if (view.phase === 'leaderboard' && phaseChanged) cues.reveal();
        }
        last.current = { phase: view.phase, sec, revealed: view.revealed };
    }, [view, sec, enabled]);
}
