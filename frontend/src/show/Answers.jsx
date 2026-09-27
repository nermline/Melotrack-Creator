import { useLayoutEffect, useRef } from 'react';
import { motion } from 'framer-motion';

// "Artist — Song" is shown as the song in bold with the artist underneath.
function splitAnswer(answer) {
    const m = (answer || '').match(/^(.+?)\s+[—–-]\s+(.+)$/);
    return m ? { title: m[2].trim(), artist: m[1].trim() } : { title: (answer || '').trim(), artist: '' };
}

// useUniformFit gives every text box inside `ref` the same font size: the
// largest size (starting at `max`, any CSS length) at which all of them fit.
// Words break only if even the smallest size does not fit.
function useUniformFit(ref, max, key, minRatio = 0.38) {
    useLayoutEffect(() => {
        const root = ref.current;
        if (!root) return;
        let alive = true;
        let raf = 0;
        const fit = () => {
            if (!alive) return;
            const boxes = [...root.querySelectorAll('[data-fit]')];
            if (!boxes.length) return;
            // The inner block's own height is measured, so centring cannot hide overflow.
            const overflows = (box) => {
                const inner = box.firstElementChild;
                return inner.offsetHeight > box.clientHeight + 1 || inner.scrollWidth > inner.clientWidth + 1;
            };
            let best = Infinity;
            for (const box of boxes) {
                box.firstElementChild.style.overflowWrap = 'normal';
                box.style.fontSize = max;
                const top = parseFloat(getComputedStyle(box).fontSize) || 16;
                let size = Math.min(top, best);
                box.style.fontSize = `${size}px`;
                while (overflows(box) && size > top * minRatio) {
                    size *= 0.94;
                    box.style.fontSize = `${size}px`;
                }
                best = Math.min(best, size);
            }
            for (const box of boxes) {
                box.style.fontSize = `${best}px`;
                box.firstElementChild.style.overflowWrap = overflows(box) ? 'anywhere' : 'normal';
            }
        };
        fit();
        // Web fonts arrive late and change text widths; the stage also resizes.
        document.fonts?.ready.then(fit);
        const ro = new ResizeObserver(() => {
            cancelAnimationFrame(raf);
            raf = requestAnimationFrame(fit);
        });
        ro.observe(root);
        return () => {
            alive = false;
            ro.disconnect();
            cancelAnimationFrame(raf);
        };
    }, [ref, max, key, minRatio]);
}

function FitBox({ center = false, style, children }) {
    return (
        <div
            data-fit=""
            style={{ overflow: 'hidden', display: 'flex', flexDirection: 'column', justifyContent: center ? 'center' : 'flex-start', ...style }}
        >
            <div>{children}</div>
        </div>
    );
}

function AnswerText({ answer }) {
    const { title, artist } = splitAnswer(answer);
    return (
        <>
            <div style={{ fontWeight: 800, lineHeight: 1.12, textWrap: 'balance' }}>{title}</div>
            {artist && (
                <div style={{ marginTop: '0.18em', fontSize: '0.74em', fontWeight: 600, lineHeight: 1.15, color: 'var(--dim)', textWrap: 'balance' }}>
                    {artist}
                </div>
            )}
        </>
    );
}

function Badge({ n, size = '5.4cqmin' }) {
    return (
        <span
            className="display"
            style={{
                position: 'absolute',
                top: '0.8cqmin',
                left: '0.8cqmin',
                minWidth: size,
                height: size,
                display: 'grid',
                placeItems: 'center',
                borderRadius: '99cqmin',
                background: 'var(--a1)',
                color: 'var(--bg)',
                fontSize: `calc(${size} * 0.6)`,
                fontStyle: 'normal',
                boxShadow: '0 0.3cqmin 1cqmin rgb(0 0 0 / 0.4)',
            }}
        >
            {n}
        </span>
    );
}

function Cover({ url, n, badge, style }) {
    return (
        <div style={{ position: 'relative', aspectRatio: '1', borderRadius: '1.2cqmin', overflow: 'hidden', background: 'rgb(0 0 0 / .3)', flexShrink: 0, ...style }}>
            {url && <img src={url} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />}
            <Badge n={n} size={badge} />
        </div>
    );
}

const enter = (i) => ({
    initial: { opacity: 0, y: '4cqmin', rotateX: -40 },
    animate: { opacity: 1, y: 0, rotateX: 0, transition: { delay: 0.25 + i * 0.12, type: 'spring', stiffness: 160, damping: 18 } },
});

// Tiles: cover on top, answer below. Fits when answers are short.
function Tiles({ list }) {
    const ref = useRef(null);
    useUniformFit(ref, 'calc(var(--cell) * 0.13)', list.map((a) => a.answer).join('|'));
    const n = list.length;
    const cols = n <= 5 ? Math.max(n, 1) : n <= 6 ? 3 : n <= 8 ? 4 : n <= 10 ? 5 : n <= 12 ? 6 : Math.ceil(n / 3);
    const rows = Math.ceil(n / cols);
    // The text box is 40% of the cover height; 1.46 = cover + text + padding.
    const cell = `min(${(88 / cols - 1.5).toFixed(2)}cqw, ${((71 - (rows - 1) * 1.8) / rows / 1.46).toFixed(2)}cqh)`;
    return (
        <div
            ref={ref}
            style={{
                '--cell': cell,
                flex: 1,
                display: 'grid',
                gridTemplateColumns: `repeat(${cols}, var(--cell))`,
                gap: '1.8cqmin',
                alignContent: 'center',
                justifyContent: 'center',
                marginTop: '2cqmin',
            }}
        >
            {list.map((a, i) => (
                <motion.div key={a.id} className="card" style={{ padding: '0.8cqmin', textAlign: 'left' }} {...enter(i)}>
                    <Cover url={a.image_url} n={i + 1} />
                    <FitBox style={{ marginTop: '0.8cqmin', height: 'calc(var(--cell) * 0.4)' }}>
                        <AnswerText answer={a.answer} />
                    </FitBox>
                </motion.div>
            ))}
        </div>
    );
}

// Rows: cover on the left and a wide text box — for long titles.
function Rows({ list }) {
    const ref = useRef(null);
    useUniformFit(ref, 'calc(var(--row) * 0.3)', list.map((a) => a.answer).join('|'));
    const n = list.length;
    const cols = n <= 4 ? 1 : n <= 12 ? 2 : 3;
    const rows = Math.ceil(n / cols);
    const h = `min(${cols === 1 ? 17 : 20}cqh, ${((68 - (rows - 1) * 1.6) / rows).toFixed(2)}cqh)`;
    const w = cols === 1 ? '72cqw' : `${(88 / cols - 1).toFixed(2)}cqw`;
    return (
        <div
            ref={ref}
            style={{
                '--row': h,
                flex: 1,
                display: 'grid',
                gridTemplateColumns: `repeat(${cols}, ${w})`,
                gridAutoFlow: cols > 1 ? 'column' : 'row',
                gridTemplateRows: `repeat(${rows}, var(--row))`,
                gap: '1.6cqmin 2cqmin',
                alignContent: 'center',
                justifyContent: 'center',
                marginTop: '2cqmin',
            }}
        >
            {list.map((a, i) => (
                <motion.div
                    key={a.id}
                    className="card"
                    style={{ padding: '0.8cqmin', display: 'flex', gap: '2cqmin', alignItems: 'center', textAlign: 'left', minWidth: 0 }}
                    {...enter(i)}
                >
                    <Cover url={a.image_url} n={i + 1} badge="min(5.4cqmin, calc(var(--row) * 0.3))" style={{ height: '100%' }} />
                    <FitBox center style={{ flex: 1, minWidth: 0, height: '100%', paddingRight: '1cqmin' }}>
                        <AnswerText answer={a.answer} />
                    </FitBox>
                </motion.div>
            ))}
        </div>
    );
}

export default function Answers({ view }) {
    const list = view.answers || [];
    // Many answers with long names get a row layout: far more room for text.
    const long = list.some((a) => Math.max(...Object.values(splitAnswer(a.answer)).map((s) => s.length)) > 26);
    const useRows = long && list.length >= 5;
    return (
        <div className="layer" style={{ justifyContent: 'flex-start', paddingTop: '5cqmin' }}>
            <motion.div
                className="display title-m"
                initial={{ opacity: 0, y: '3cqmin' }}
                animate={{ opacity: 1, y: 0, transition: { duration: 0.6 } }}
            >
                {view.category?.title}
            </motion.div>
            <div className="sub" style={{ marginTop: '0.8cqmin' }}>
                Правильні відповіді
            </div>
            {useRows ? <Rows list={list} /> : <Tiles list={list} />}
        </div>
    );
}
