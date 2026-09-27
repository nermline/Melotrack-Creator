import { useCallback, useEffect, useRef, useState } from 'react';
import { Crop, Maximize, Music, Pause, Play, Repeat, StepBack, StepForward } from 'lucide-react';
import { fmtTime } from '../lib/format';
import { Button, Segmented, Spinner } from '../ui';
import { cx } from '../lib/cx';

const clamp = (v, lo, hi) => Math.min(Math.max(v, lo), hi);
const r1 = (v) => Math.round(v * 10) / 10;

// Largest centred 16:9 rectangle inside the frame (in source pixels).
function defaultCrop(W, H) {
    let w = W;
    let h = Math.round((w * 9) / 16);
    if (h > H) {
        h = H;
        w = Math.round((h * 16) / 9);
    }
    return { x: Math.round((W - w) / 2), y: Math.round((H - h) / 2), w, h };
}

// ClipEditor picks the fragment (start/end) and the frame (whole picture or a
// 16:9 crop) on the downloaded source video.
export default function ClipEditor({ media, value, onChange }) {
    const videoRef = useRef(null);
    const [cur, setCur] = useState(value.start);
    // null when stopped, 'fragment' while previewing the clip, 'free' otherwise.
    const [playMode, setPlayMode] = useState(null);
    const [loop, setLoop] = useState(false);
    const [loading, setLoading] = useState(true);
    const dur = media.duration || 0;
    const hasVideo = media.width > 0 && media.height > 0;
    const len = value.end - value.start;

    const set = useCallback((patch) => onChange({ ...value, ...patch }), [value, onChange]);

    const setRange = useCallback(
        (start, end) => {
            start = clamp(r1(start), 0, Math.max(dur - 1, 0));
            end = clamp(r1(end), start + 1, dur || start + 600);
            set({ start, end });
        },
        [dur, set],
    );

    // Keep the playhead inside the fragment while previewing it.
    useEffect(() => {
        const v = videoRef.current;
        if (!v) return;
        const onTime = () => {
            setCur(v.currentTime);
            if (playMode === 'fragment' && v.currentTime >= value.end) {
                if (loop) v.currentTime = value.start;
                else {
                    v.pause();
                    v.currentTime = value.start;
                }
            }
        };
        const onPause = () => setPlayMode(null);
        v.addEventListener('timeupdate', onTime);
        v.addEventListener('pause', onPause);
        return () => {
            v.removeEventListener('timeupdate', onTime);
            v.removeEventListener('pause', onPause);
        };
    }, [value.start, value.end, loop, playMode]);

    const seek = (t) => {
        const v = videoRef.current;
        if (!v) return;
        v.currentTime = clamp(t, 0, dur);
        setCur(v.currentTime);
    };

    const playFragment = () => {
        const v = videoRef.current;
        if (!v) return;
        if (playMode) {
            v.pause();
            return;
        }
        setPlayMode('fragment');
        v.currentTime = value.start;
        v.play().catch(() => setPlayMode(null));
    };

    const playFromHere = () => {
        const v = videoRef.current;
        if (!v) return;
        if (playMode) v.pause();
        else {
            setPlayMode('free');
            v.play().catch(() => setPlayMode(null));
        }
    };

    // Space plays the fragment (unless typing).
    useEffect(() => {
        const onKey = (e) => {
            if (e.code !== 'Space' || /INPUT|TEXTAREA|SELECT|BUTTON/.test(e.target.tagName)) return;
            e.preventDefault();
            playFragment();
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    });

    const frame = hasVideo ? value.frame : 'fit';
    const crop = value.crop || (hasVideo ? defaultCrop(media.width, media.height) : null);

    // Detail view: the fragment with some context on both sides.
    const pad = Math.max(5, len * 0.6);
    const viewStart = Math.max(0, value.start - pad);
    const viewEnd = Math.min(dur, value.end + pad);

    return (
        <div>
            <div
                className={cx(
                    'relative mx-auto overflow-hidden rounded-xl bg-black select-none',
                    frame === 'crop' ? '' : 'aspect-video',
                )}
                style={frame === 'crop' ? { aspectRatio: `${media.width} / ${media.height}`, maxHeight: '52vh' } : { maxHeight: '52vh' }}
            >
                <video
                    ref={videoRef}
                    src={media.source_url}
                    preload="auto"
                    playsInline
                    onLoadedMetadata={() => {
                        setLoading(false);
                        seek(value.start);
                    }}
                    onWaiting={() => setLoading(true)}
                    onCanPlay={() => setLoading(false)}
                    className={cx('absolute inset-0 h-full w-full', frame === 'crop' ? 'object-fill' : 'object-contain')}
                />
                {!hasVideo && (
                    <div className="absolute inset-0 grid place-items-center text-dim">
                        <div className="flex flex-col items-center gap-2">
                            <Music size={40} />
                            <span className="text-sm">Лише аудіо — на екрані буде тема оформлення</span>
                        </div>
                    </div>
                )}
                {frame === 'crop' && crop && (
                    <CropBox media={media} crop={crop} onChange={(c) => set({ crop: c })} />
                )}
                {loading && (
                    <div className="absolute top-3 right-3 rounded-full bg-black/60 p-1.5 text-white">
                        <Spinner size={16} />
                    </div>
                )}
            </div>

            <div className="mt-4 flex flex-wrap items-center gap-2">
                <Button variant="primary" icon={playMode === 'fragment' ? Pause : Play} onClick={playFragment}>
                    {playMode === 'fragment' ? 'Пауза' : 'Прослухати фрагмент'}
                </Button>
                <Button
                    icon={Repeat}
                    variant={loop ? 'primary' : undefined}
                    className={loop ? '' : 'text-dim'}
                    onClick={() => setLoop((l) => !l)}
                    title="Повторювати фрагмент по колу"
                />
                <Button variant="ghost" icon={playMode === 'free' ? Pause : Play} onClick={playFromHere}>
                    З поточного місця
                </Button>
                <span className="ml-auto font-mono text-sm text-dim">
                    {fmtTime(cur, true)} / {fmtTime(dur)}
                </span>
            </div>

            <div className="mt-4">
                <Timeline
                    viewStart={0}
                    viewEnd={dur}
                    start={value.start}
                    end={value.end}
                    cur={cur}
                    onRange={setRange}
                    onSeek={seek}
                    height={34}
                />
                <div className="mt-3 text-xs text-faint">Точне налаштування</div>
                <Timeline
                    viewStart={viewStart}
                    viewEnd={viewEnd}
                    start={value.start}
                    end={value.end}
                    cur={cur}
                    onRange={setRange}
                    onSeek={seek}
                    height={52}
                    ticks
                />
            </div>

            <div className="mt-4 grid gap-3 sm:grid-cols-2">
                <div className="panel bg-panel-2 p-3">
                    <div className="label">Початок</div>
                    <div className="flex items-center gap-1.5">
                        <Button size="sm" icon={StepBack} onClick={() => setRange(value.start - 0.5, value.end - 0.5)} title="−0,5 с" />
                        <input
                            type="number"
                            step="0.1"
                            min="0"
                            className="field w-24 py-1 text-center font-mono"
                            value={value.start}
                            onChange={(e) => {
                                const s = Number(e.target.value) || 0;
                                setRange(s, s + len);
                            }}
                        />
                        <Button size="sm" icon={StepForward} onClick={() => setRange(value.start + 0.5, value.end + 0.5)} title="+0,5 с" />
                        <Button size="sm" variant="ghost" onClick={() => setRange(cur, cur + len)} title="Перенести фрагмент на поточний момент">
                            сюди
                        </Button>
                    </div>
                </div>
                <div className="panel bg-panel-2 p-3">
                    <div className="label">Тривалість, с</div>
                    <div className="flex flex-wrap items-center gap-1.5">
                        <input
                            type="number"
                            step="0.5"
                            min="1"
                            className="field w-20 py-1 text-center font-mono"
                            value={r1(len)}
                            onChange={(e) => setRange(value.start, value.start + (Number(e.target.value) || 1))}
                        />
                        {[10, 15, 20, 30].map((n) => (
                            <Button
                                key={n}
                                size="sm"
                                variant={Math.abs(len - n) < 0.05 ? 'primary' : 'ghost'}
                                onClick={() => setRange(value.start, value.start + n)}
                            >
                                {n}
                            </Button>
                        ))}
                        <Button size="sm" variant="ghost" onClick={() => cur > value.start + 1 && setRange(value.start, cur)} title="Кінець на поточному моменті">
                            кінець тут
                        </Button>
                    </div>
                </div>
            </div>

            {hasVideo && (
                <div className="mt-4 flex flex-wrap items-center gap-3">
                    <span className="label m-0">Кадр</span>
                    <Segmented
                        value={frame}
                        onChange={(f) => set({ frame: f, crop: f === 'crop' ? crop : null })}
                        options={[
                            { value: 'fit', label: 'Увесь кадр', icon: Maximize },
                            { value: 'crop', label: 'Обрізати 16:9', icon: Crop },
                        ]}
                    />
                    <span className="text-xs text-faint">
                        {frame === 'fit'
                            ? 'Картинка вписується в екран, зайве — чорні поля'
                            : 'Перетягніть рамку, кут — змінити розмір'}
                    </span>
                </div>
            )}
        </div>
    );
}

// Timeline shows [viewStart, viewEnd] with the fragment and its handles.
function Timeline({ viewStart, viewEnd, start, end, cur, onRange, onSeek, height, ticks }) {
    const ref = useRef(null);
    const span = Math.max(viewEnd - viewStart, 0.001);
    const pct = (t) => `${((t - viewStart) / span) * 100}%`;

    const timeAt = (clientX) => {
        const r = ref.current.getBoundingClientRect();
        return viewStart + clamp((clientX - r.left) / r.width, 0, 1) * span;
    };

    const onPointerDown = (e) => {
        const r = ref.current.getBoundingClientRect();
        const x = e.clientX - r.left;
        const xs = ((start - viewStart) / span) * r.width;
        const xe = ((end - viewStart) / span) * r.width;
        const t0 = timeAt(e.clientX);
        let mode = 'seek';
        if (Math.abs(x - xs) <= 10) mode = 'start';
        else if (Math.abs(x - xe) <= 10) mode = 'end';
        else if (x > xs && x < xe) mode = 'move';
        e.currentTarget.setPointerCapture(e.pointerId);
        const s0 = start;
        const e0 = end;
        const last = { s: s0, e: e0 };
        const move = (ev) => {
            const t = timeAt(ev.clientX);
            if (mode === 'start') [last.s, last.e] = [Math.min(t, e0 - 1), e0];
            else if (mode === 'end') [last.s, last.e] = [s0, Math.max(t, s0 + 1)];
            else if (mode === 'move') [last.s, last.e] = [s0 + (t - t0), e0 + (t - t0)];
            else return onSeek(t);
            onRange(last.s, last.e);
        };
        if (mode === 'seek') move(e);
        const el = e.currentTarget;
        const up = () => {
            el.removeEventListener('pointermove', move);
            el.removeEventListener('pointerup', up);
            // Park the playhead where the change can be heard right away.
            if (mode === 'end') onSeek(Math.max(last.e - 3, last.s));
            else if (mode !== 'seek') onSeek(Math.max(last.s, 0));
        };
        el.addEventListener('pointermove', move);
        el.addEventListener('pointerup', up);
    };

    const tickStep = span > 120 ? 30 : span > 40 ? 10 : span > 15 ? 5 : 1;
    const tickList = [];
    if (ticks) {
        for (let t = Math.ceil(viewStart / tickStep) * tickStep; t <= viewEnd; t += tickStep) tickList.push(t);
    }

    return (
        <div
            ref={ref}
            onPointerDown={onPointerDown}
            className="relative cursor-pointer touch-none overflow-hidden rounded-lg bg-white/[0.06]"
            style={{ height }}
        >
            {tickList.map((t) => (
                <div key={t} className="absolute top-0 bottom-0 border-l border-white/10" style={{ left: pct(t) }}>
                    <span className="absolute top-0.5 left-1 font-mono text-[10px] text-faint">{fmtTime(t)}</span>
                </div>
            ))}
            <div
                className="absolute top-0 bottom-0 border-y-2 border-accent bg-accent/25"
                style={{ left: pct(start), width: `${((end - start) / span) * 100}%`, cursor: 'grab' }}
            />
            {[start, end].map((t, i) => (
                <div
                    key={i}
                    className="absolute top-0 bottom-0 w-1.5 -translate-x-1/2 rounded-full bg-accent"
                    style={{ left: pct(t), cursor: 'ew-resize' }}
                />
            ))}
            {cur >= viewStart && cur <= viewEnd && (
                <div className="pointer-events-none absolute top-0 bottom-0 w-0.5 bg-accent-2" style={{ left: pct(cur) }} />
            )}
        </div>
    );
}

// CropBox is a draggable, resizable 16:9 rectangle over the source video.
function CropBox({ media, crop, onChange }) {
    const W = media.width;
    const H = media.height;
    const box = useRef(null);

    const start = (e, mode) => {
        e.preventDefault();
        e.stopPropagation();
        const parent = box.current.parentElement.getBoundingClientRect();
        const sx = W / parent.width;
        const sy = H / parent.height;
        const c0 = { ...crop };
        const x0 = e.clientX;
        const y0 = e.clientY;
        const move = (ev) => {
            const dx = (ev.clientX - x0) * sx;
            const dy = (ev.clientY - y0) * sy;
            if (mode === 'move') {
                onChange({ ...c0, x: Math.round(clamp(c0.x + dx, 0, W - c0.w)), y: Math.round(clamp(c0.y + dy, 0, H - c0.h)) });
            } else {
                let w = clamp(c0.w + dx, W * 0.15, W - c0.x);
                let h = (w * 9) / 16;
                if (c0.y + h > H) {
                    h = H - c0.y;
                    w = (h * 16) / 9;
                }
                onChange({ ...c0, w: Math.round(w), h: Math.round(h) });
            }
        };
        const up = () => {
            window.removeEventListener('pointermove', move);
            window.removeEventListener('pointerup', up);
        };
        window.addEventListener('pointermove', move);
        window.addEventListener('pointerup', up);
    };

    return (
        <div
            ref={box}
            onPointerDown={(e) => start(e, 'move')}
            className="absolute cursor-move touch-none border-2 border-accent-2"
            style={{
                left: `${(crop.x / W) * 100}%`,
                top: `${(crop.y / H) * 100}%`,
                width: `${(crop.w / W) * 100}%`,
                height: `${(crop.h / H) * 100}%`,
                boxShadow: '0 0 0 9999px rgb(0 0 0 / 0.6)',
            }}
        >
            <span className="absolute top-1 left-1.5 rounded bg-black/60 px-1.5 text-[11px] font-bold text-accent-2">16:9</span>
            <span
                onPointerDown={(e) => start(e, 'resize')}
                className="absolute -right-2 -bottom-2 h-5 w-5 cursor-nwse-resize rounded-md border-2 border-bg bg-accent-2"
            />
        </div>
    );
}
