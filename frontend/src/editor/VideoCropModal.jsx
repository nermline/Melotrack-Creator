import { useEffect, useRef, useState, useCallback } from 'react';
import { Modal, Button, Field, NumberInput, Segmented } from '../ui/kit';

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
const fmt = (s) => {
    if (!isFinite(s)) return '0:00';
    const m = Math.floor(s / 60);
    const sec = Math.floor(s % 60);
    return `${m}:${String(sec).padStart(2, '0')}`;
};

export default function VideoCropModal({ open, videoUrl, mediaW, mediaH, duration, initial, onCancel, onConfirm }) {
    const haveMeta = mediaW > 0 && mediaH > 0 && duration > 0;

    const hNorm = useCallback((w) => (w * mediaW * 9) / (16 * mediaH || 1), [mediaW, mediaH]);
    const defaultCrop = useCallback(() => {
        const videoRatio = mediaW / mediaH;
        const w = videoRatio >= 16 / 9 ? (mediaH * 16 / 9) / mediaW : 1;
        return { x: (1 - w) / 2, y: (1 - hNorm(w)) / 2, w };
    }, [mediaW, mediaH, hNorm]);

    const [mode, setMode] = useState('crop');
    const [crop, setCrop] = useState({ x: 0, y: 0, w: 1 });
    const [dur, setDur] = useState(10);
    const [start, setStart] = useState(0);
    const [playing, setPlaying] = useState(false);
    const [cur, setCur] = useState(0);

    const videoRef = useRef(null);
    const frameRef = useRef(null);

    useEffect(() => {
        if (!open || !haveMeta) return;
        const initDur = initial?.end > initial?.start && initial?.end > 0
            ? Math.round((initial.end - initial.start) * 10) / 10
            : Math.min(10, Math.floor(duration));
        const d = clamp(initDur || 10, 1, Math.floor(duration));
        setDur(d);
        setStart(clamp(initial?.start || 0, 0, Math.max(0, duration - d)));
        setMode(initial?.fit ? 'fit' : 'crop');

        const valid = initial && initial.cropW > 0 && initial.cropH > 0
            && initial.cropX + initial.cropW <= mediaW && initial.cropY + initial.cropH <= mediaH;
        setCrop(valid ? { x: initial.cropX / mediaW, y: initial.cropY / mediaH, w: initial.cropW / mediaW } : defaultCrop());
        setPlaying(false);
        setCur(initial?.start || 0);
    }, [open, haveMeta, duration, mediaW, mediaH, initial, defaultCrop]);

    const maxStart = Math.max(0, duration - dur);
    useEffect(() => { setStart((s) => clamp(s, 0, maxStart)); }, [maxStart]);

    useEffect(() => {
        const v = videoRef.current;
        if (!v || !open || playing) return;
        const apply = () => { try { v.currentTime = start; setCur(start); } catch {} };
        if (v.readyState >= 1) apply(); else v.addEventListener('loadedmetadata', apply, { once: true });
    }, [start, open, playing]);

    useEffect(() => {
        const v = videoRef.current;
        if (!v || !open) return;
        const onTime = () => {
            setCur(v.currentTime);
            if (v.currentTime >= start + dur) v.currentTime = start;
        };
        v.addEventListener('timeupdate', onTime);
        return () => v.removeEventListener('timeupdate', onTime);
    }, [start, dur, open]);

    const togglePlay = () => {
        const v = videoRef.current;
        if (!v) return;
        if (playing) { v.pause(); setPlaying(false); }
        else { v.currentTime = start; v.play().then(() => setPlaying(true)).catch(() => {}); }
    };

    const dragRef = useRef(null);
    const onFramePointerDown = (e, m) => {
        e.preventDefault(); e.stopPropagation();
        const rect = frameRef.current.getBoundingClientRect();
        dragRef.current = { mode: m, rect, startX: e.clientX, startY: e.clientY, crop: { ...crop } };
        window.addEventListener('pointermove', onPointerMove);
        window.addEventListener('pointerup', onPointerUp, { once: true });
    };
    const onPointerMove = (e) => {
        const d = dragRef.current; if (!d) return;
        const dx = (e.clientX - d.startX) / d.rect.width;
        const dy = (e.clientY - d.startY) / d.rect.height;
        if (d.mode === 'move') {
            const h = hNorm(d.crop.w);
            setCrop({ w: d.crop.w, x: clamp(d.crop.x + dx, 0, 1 - d.crop.w), y: clamp(d.crop.y + dy, 0, 1 - h) });
        } else {
            let w = clamp(d.crop.w + dx, 0.12, 1 - d.crop.x);
            const maxWByHeight = ((1 - d.crop.y) * mediaH * 16) / (mediaW * 9);
            w = Math.min(w, maxWByHeight);
            setCrop({ ...d.crop, w });
        }
    };
    const onPointerUp = () => { dragRef.current = null; window.removeEventListener('pointermove', onPointerMove); };

    const confirm = () => {
        const base = { start_time: Math.round(start * 10) / 10, end_time: Math.round((start + dur) * 10) / 10 };
        if (mode === 'fit') {
            onConfirm({ ...base, fit: true, crop_x: 0, crop_y: 0, crop_width: 0, crop_height: 0 });
        } else {
            const cropX = Math.round(crop.x * mediaW);
            const cropY = Math.round(crop.y * mediaH);
            const cropW = Math.round(crop.w * mediaW);
            let cropH = Math.round((cropW * 9) / 16);
            if (cropY + cropH > mediaH) cropH = mediaH - cropY;
            onConfirm({ ...base, fit: false, crop_x: cropX, crop_y: cropY, crop_width: cropW, crop_height: cropH });
        }
    };

    const h = hNorm(crop.w);
    const isFit = mode === 'fit';
    const containerPct = isFit ? 56.25 : (haveMeta ? (mediaH / mediaW) * 100 : 56.25);

    return (
        <Modal open={open} onClose={onCancel} title="Налаштувати відео" maxWidth={860}
            footer={<><Button variant="ghost" onClick={onCancel}>Скасувати</Button>
                <Button variant="primary" onClick={confirm} disabled={!haveMeta}>Застосувати</Button></>}>
            {!haveMeta ? (
                <p style={{ color: 'var(--color-muted)' }}>Метадані відео ще не готові (триває завантаження). Спробуйте трохи пізніше.</p>
            ) : (
                <>
                    <div className="mb-3 flex items-center justify-between gap-3 flex-wrap">
                        <Segmented value={mode} onChange={setMode}
                            options={[{ value: 'crop', label: '▣ Обрізати 16:9' }, { value: 'fit', label: '⊡ Вмістити 16:9' }]} />
                        <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
                            {isFit ? 'усе відео з чорними полями (без втрат)' : 'частина кадру обріжеться'}
                        </span>
                    </div>

                    <div ref={frameRef}
                        style={{ position: 'relative', width: '100%', paddingTop: `${containerPct}%`,
                            background: '#000', borderRadius: 12, overflow: 'hidden', userSelect: 'none' }}>
                        <video ref={videoRef} src={videoUrl} playsInline
                            style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: isFit ? 'contain' : 'fill' }}
                            onPause={() => setPlaying(false)} />

                        {!isFit && (
                            <div onPointerDown={(e) => onFramePointerDown(e, 'move')}
                                style={{ position: 'absolute', left: `${crop.x * 100}%`, top: `${crop.y * 100}%`,
                                    width: `${crop.w * 100}%`, height: `${h * 100}%`,
                                    border: '2px solid var(--color-accent)', boxShadow: '0 0 0 9999px rgba(0,0,0,0.55)',
                                    cursor: 'move', borderRadius: 2 }}>
                                <span style={{ position: 'absolute', top: 4, left: 6, fontSize: 11, color: '#fff', textShadow: '0 1px 2px #000' }}>16:9</span>
                                <div onPointerDown={(e) => onFramePointerDown(e, 'resize')}
                                    style={{ position: 'absolute', right: -7, bottom: -7, width: 16, height: 16,
                                        background: 'var(--color-accent)', borderRadius: 4, cursor: 'nwse-resize', border: '2px solid #fff' }} />
                            </div>
                        )}
                    </div>

                    <div className="mt-5">
                        <div className="flex items-center justify-between mb-1 text-sm" style={{ color: 'var(--color-muted)' }}>
                            <span>Кадр зараз: <b style={{ color: 'var(--color-text)' }}>{fmt(cur)}</b></span>
                            <span>Відрізок {fmt(start)} – {fmt(start + dur)} · усе відео {fmt(duration)}</span>
                        </div>
                        <Timeline duration={duration} start={start} dur={dur} maxStart={maxStart} playhead={cur} onSeek={setStart} />
                    </div>

                    <div className="mt-4 flex flex-wrap items-end gap-4">
                        <Field label="Тривалість кліпу, с" className="w-40">
                            <NumberInput min={1} max={Math.floor(duration)} step={0.5} value={dur}
                                onChange={(e) => setDur(clamp(Number(e.target.value) || 1, 1, Math.floor(duration)))} />
                        </Field>
                        <Field label={`Початок, с (макс ${maxStart.toFixed(1)})`} className="w-44">
                            <NumberInput min={0} max={maxStart} step={0.1} value={start}
                                onChange={(e) => setStart(clamp(Number(e.target.value) || 0, 0, maxStart))} />
                        </Field>
                        <Button onClick={togglePlay}>{playing ? '⏸ Пауза' : '▶ Прев’ю відрізка'}</Button>
                        {!isFit && <Button variant="ghost" onClick={() => setCrop(defaultCrop())}>↺ Скинути рамку</Button>}
                    </div>
                </>
            )}
        </Modal>
    );
}

function Timeline({ duration, start, dur, maxStart, playhead, onSeek }) {
    const barRef = useRef(null);
    const dragging = useRef(false);
    const seekFromEvent = (clientX) => {
        const r = barRef.current.getBoundingClientRect();
        const t = clamp((clientX - r.left) / r.width, 0, 1) * duration;
        onSeek(clamp(t, 0, maxStart));
    };
    const onDown = (e) => {
        dragging.current = true; seekFromEvent(e.clientX);
        const move = (ev) => dragging.current && seekFromEvent(ev.clientX);
        const up = () => { dragging.current = false; window.removeEventListener('pointermove', move); };
        window.addEventListener('pointermove', move);
        window.addEventListener('pointerup', up, { once: true });
    };
    const pct = (t) => `${(t / duration) * 100}%`;
    return (
        <div ref={barRef} onPointerDown={onDown}
            style={{ position: 'relative', height: 28, borderRadius: 8, cursor: 'pointer', background: 'rgba(255,255,255,0.08)' }}>
            <div style={{ position: 'absolute', inset: 0, left: 0, width: pct(maxStart), background: 'rgba(68,208,123,0.14)', borderRadius: 8 }} />
            <div style={{ position: 'absolute', top: 0, bottom: 0, left: pct(start), width: pct(dur),
                background: 'linear-gradient(180deg, rgba(124,131,255,0.55), rgba(108,114,245,0.4))', borderRadius: 8 }} />
            <div style={{ position: 'absolute', top: -3, bottom: -3, left: pct(playhead), width: 2,
                background: 'var(--color-accent2)', boxShadow: '0 0 6px var(--color-accent2)' }} />
            <div style={{ position: 'absolute', top: '50%', left: pct(start), transform: 'translate(-50%,-50%)',
                width: 14, height: 14, borderRadius: '50%', background: '#fff', boxShadow: '0 1px 4px rgba(0,0,0,0.6)' }} />
        </div>
    );
}
