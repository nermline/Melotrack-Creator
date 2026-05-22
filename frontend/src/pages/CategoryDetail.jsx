import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { motion, AnimatePresence, Reorder, useDragControls } from 'framer-motion';
import api from '../api';
import { Button, Glass, Badge, Field, TextInput, Toggle, Spinner } from '../ui/kit';
import ImageCropModal from '../editor/ImageCropModal';
import VideoCropModal from '../editor/VideoCropModal';

// ─── Попередження про незбережені зміни (при виході зі сторінки) ────────────
function useUnsavedWarning(dirty) {
    useEffect(() => {
        if (!dirty) return;
        const handler = (e) => { e.preventDefault(); e.returnValue = ''; };
        window.addEventListener('beforeunload', handler);
        return () => window.removeEventListener('beforeunload', handler);
    }, [dirty]);
}

// ─── Константи / хелпери ────────────────────────────────────────────────────
const API_BASE = '';
const WS_BASE  = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}`;
const SYNC_THROTTLE_MS = 50;

function extractYouTubeId(url) {
    if (!url) return null;
    const m = url.match(/(?:v=|youtu\.be\/|embed\/)([a-zA-Z0-9_-]{11})/);
    return m ? m[1] : null;
}
function cleanYouTubeUrl(url) {
    if (!url) return url;
    const id = extractYouTubeId(url);
    return id ? `https://www.youtube.com/watch?v=${id}` : url;
}
function hasPlaylistParam(url) {
    return Boolean(url && (url.includes('&list=') || url.includes('?list=')));
}
function fmtDur(secs) {
    if (!secs) return '—';
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${String(s).padStart(2, '0')}`;
}

function validateCropForm(form, media) {
    const errors = {};
    const start = parseFloat(form.start_time) || 0;
    const end   = parseFloat(form.end_time)   || 0;
    if (start < 0) errors.start_time = 'Не може бути від\'ємним';
    if (end > 0 && end <= start) errors.end_time = `Має бути > ${start.toFixed(1)}s`;
    const dur = media?.duration ?? 0;
    if (dur > 0) {
        if (!errors.start_time && start >= dur) errors.start_time = `≥ тривалість (${dur.toFixed(1)}s)`;
        if (!errors.end_time && end > dur) errors.end_time = `> тривалість (${dur.toFixed(1)}s)`;
    }
    return errors;
}

function itemToForm(item) {
    return {
        show_video:   item.show_video ?? false,
        youtube_url:  item.video?.youtube_url ?? '',
        start_time:   item.video?.start_time ?? 0,
        end_time:     item.video?.end_time ?? 30,
        volume:       item.video?.volume ?? 1,
        crop_x:       item.video?.crop_x ?? 0,
        crop_y:       item.video?.crop_y ?? 0,
        crop_width:   item.video?.crop_width ?? 0,
        crop_height:  item.video?.crop_height ?? 0,
        fit:          item.video?.fit ?? false,
        answer_title: item.answer?.title ?? '',
    };
}
function toPayload(f) {
    return {
        show_video: f.show_video,
        video: {
            youtube_url: cleanYouTubeUrl(f.youtube_url),
            start_time:  parseFloat(f.start_time) || 0,
            end_time:    parseFloat(f.end_time)   || 0,
            volume:      parseFloat(f.volume)     || 1,
            crop_x:      parseInt(f.crop_x)       || 0,
            crop_y:      parseInt(f.crop_y)       || 0,
            crop_width:  parseInt(f.crop_width)   || 0,
            crop_height: parseInt(f.crop_height)  || 0,
            fit:         !!f.fit,
        },
        answer: { title: f.answer_title },
    };
}

// Чи відрізняється чернетка від збереженого стану (включно зі стейджем фото).
function isDraftDirty(item, draft) {
    if (!draft) return false;
    if (draft._imageFile || draft._imageDelete) return true;
    const base = itemToForm(item);
    return Object.keys(base).some((k) => String(base[k]) !== String(draft[k]));
}

function mediaBadge(s) {
    const m = { ready: ['ok', '✓'], downloading: ['warn', '⏳'], error: ['danger', '✕'] };
    return m[s] || ['muted', '—'];
}
function renderBadge(s) {
    const m = { ready: ['ok', '✓ кліп'], rendering: ['warn', '⏳ рендер'], unrendered: ['muted', 'без кліпу'], error: ['danger', '✕'] };
    return m[s] || ['muted', '—'];
}

// ─── WS-хук редактора ───────────────────────────────────────────────────────
function useEditorWS(cid, onMessage) {
    const [wsStatus, setWsStatus] = useState('connecting');
    const wsRef = useRef(null);
    const onMessageRef = useRef(onMessage);
    useEffect(() => { onMessageRef.current = onMessage; }, [onMessage]);
    useEffect(() => {
        if (!cid) return;
        let alive = true, reconnectTimer = null;
        function connect() {
            if (!alive) return;
            const token = localStorage.getItem('token');
            const ws = new WebSocket(`${WS_BASE}/api/categories/${cid}/ws?token=${token}`);
            wsRef.current = ws;
            setWsStatus('connecting');
            ws.onopen  = () => { if (alive) setWsStatus('connected'); };
            ws.onerror = () => {};
            ws.onclose = () => { if (!alive) return; setWsStatus('disconnected'); reconnectTimer = setTimeout(connect, 3000); };
            ws.onmessage = (e) => { try { onMessageRef.current(JSON.parse(e.data)); } catch { /* ignore */ } };
        }
        connect();
        return () => { alive = false; clearTimeout(reconnectTimer); wsRef.current?.close(); };
    }, [cid]);
    const sendMessage = useCallback((msg) => {
        const ws = wsRef.current;
        if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
    }, []);
    return { wsStatus, sendMessage };
}

// Плейсхолдер для рамки без завантаженого фото (значок + підпис).
function EditorPhotoPlaceholder({ label = 'нема фото', size = 28 }) {
    return (
        <div className="flex flex-col items-center justify-center gap-1" style={{ color: 'var(--color-muted)' }}>
            <span style={{ fontSize: size, lineHeight: 1, opacity: 0.7 }}>🖼</span>
            <span className="text-xs">{label}</span>
        </div>
    );
}

// ─── Рядок списку (зліва, з drag-реордером) ─────────────────────────────────
function ItemRow({ item, index, selected, dirty, onSelect, onRender, onRetry, onCommit }) {
    const controls = useDragControls();
    const token = localStorage.getItem('token');
    const img = item.answer?.image_path ? `${API_BASE}${item.answer.image_path}?token=${token}` : null;
    const [, mLabel] = mediaBadge(item.video?.media?.status);
    const [rTone] = renderBadge(item.video?.render_status);
    const mediaReady = item.video?.media?.status === 'ready';

    return (
        <Reorder.Item as="div" value={item} dragListener={false} dragControls={controls}
            onDragEnd={() => onCommit(item.id)} layout
            style={{
                borderRadius: 12, marginBottom: 8, cursor: 'pointer',
                border: selected ? '1px solid var(--color-accent)' : '1px solid rgba(255,255,255,0.10)',
                background: selected ? 'rgba(124,131,255,0.14)' : 'rgba(255,255,255,0.045)',
                boxShadow: selected ? '0 0 0 3px rgba(124,131,255,0.18)' : 'none',
            }}
            onClick={() => onSelect(item.id)}>
            <div className="flex items-center gap-2 px-2 py-2">
                {/* Ручка перетягування */}
                <div onPointerDown={(e) => controls.start(e)} title="Перетягніть, щоб змінити порядок"
                    style={{ cursor: 'grab', color: 'var(--color-muted)', padding: '0 4px', touchAction: 'none', fontSize: 18 }}
                    onClick={(e) => e.stopPropagation()}>⠿</div>
                <span className="text-xs" style={{ color: 'var(--color-muted)', minWidth: 18 }}>{index + 1}</span>
                <div style={{ width: 40, height: 40, borderRadius: 8, overflow: 'hidden', flexShrink: 0,
                    background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.1)',
                    display: 'grid', placeItems: 'center' }}>
                    {img
                        ? <img src={img} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                            onError={(e) => { e.target.style.display = 'none'; }} />
                        : <span title="нема фото" style={{ fontSize: 16, opacity: 0.5 }}>🖼</span>}
                </div>
                <div className="flex-1 min-w-0">
                    <div className="truncate font-medium" style={{ color: '#fff' }}>
                        {item.answer?.title || <em style={{ color: 'var(--color-muted)', fontWeight: 400 }}>без відповіді</em>}
                        {dirty && <span title="незбережено" style={{ color: 'var(--color-warn)' }}> ●</span>}
                    </div>
                    <div className="flex items-center gap-2 text-xs" style={{ color: 'var(--color-muted)' }}>
                        <span>{item.show_video ? '📺 відео' : '🎵 аудіо'}</span>
                        <span title="завантаження">{mLabel}</span>
                        <Badge tone={rTone}>{renderBadge(item.video?.render_status)[1]}</Badge>
                    </div>
                </div>
                {item.video?.media?.status === 'error' && (
                    <span role="button" tabIndex={0} title="Спробувати завантажити ще раз"
                        onClick={(e) => { e.stopPropagation(); onRetry(item.id); }}
                        style={{ color: 'var(--color-danger)', cursor: 'pointer', fontSize: 13, whiteSpace: 'nowrap', padding: '0 4px' }}>
                        ↻ ще раз
                    </span>
                )}
                {mediaReady && item.video?.render_status !== 'rendering' && (
                    <span role="button" tabIndex={0} title="Зробити кліп (рендер)"
                        onClick={(e) => { e.stopPropagation(); onRender(item.id); }}
                        style={{ color: 'var(--color-accent)', cursor: 'pointer', fontSize: 13, whiteSpace: 'nowrap', padding: '0 4px' }}>
                        ▶ рендер
                    </span>
                )}
            </div>
        </Reorder.Item>
    );
}

// ─── Налаштування питання (праворуч) — керована чернетка зверху ─────────────
function ItemSettings({ item, draft, onChange, pid, cid, renderVersion, imageVersion,
    onSave, onRender, onDelete, onReset, onRetry }) {
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const [filePreview, setFilePreview] = useState(null);
    const [cropFile, setCropFile] = useState(null);
    const [photoModal, setPhotoModal] = useState(false);
    const [videoModal, setVideoModal] = useState(false);

    const token = localStorage.getItem('token');
    const media = item.video?.media;
    const mediaReady = media?.status === 'ready';
    const ytId = extractYouTubeId(item.video?.youtube_url);

    const renderedVideoUrl = item.video?.render_status === 'ready'
        ? `${API_BASE}/media/${pid}/${cid}/${item.id}.mp4?token=${token}&v=${renderVersion}` : null;
    const rawVideoUrl = ytId ? `${API_BASE}/raw/${ytId}.mp4?token=${token}` : null;
    const embedStart = Math.floor(parseFloat(draft.start_time) || 0);
    const embedUrl = ytId ? `https://www.youtube.com/embed/${ytId}?start=${embedStart}&rel=0` : null;

    const serverImg = item.answer?.image_path ? `${API_BASE}${item.answer.image_path}?token=${token}&v=${imageVersion}` : null;
    const stagedFile = draft._imageFile || null;
    const stagedDelete = !!draft._imageDelete;
    const imageStaged = Boolean(stagedFile || stagedDelete);
    const displayImg = stagedDelete ? null : (filePreview || serverImg);

    const baseline = itemToForm(item);
    const isCh = (k) => String(draft[k]) !== String(baseline[k]);
    const videoChanged = ['start_time', 'end_time', 'crop_x', 'crop_y', 'crop_width', 'crop_height', 'fit'].some(isCh);

    const validationErrors = validateCropForm(draft, media);
    const hasValidationErrors = Object.keys(validationErrors).length > 0;
    const dirty = isDraftDirty(item, draft);

    useEffect(() => {
        if (!stagedFile) { setFilePreview(null); return; }
        const u = URL.createObjectURL(stagedFile);
        setFilePreview(u);
        return () => URL.revokeObjectURL(u);
    }, [stagedFile]);

    const set = (k, v) => onChange({ [k]: v });
    const setMany = (patch) => onChange(patch);
    const setImage = (patch) => onChange(patch);

    const handleSave = async () => {
        if (hasValidationErrors) return;
        setSaving(true); setError('');
        try { await onSave(item.id, draft); } catch (e) { setError(e.response?.data?.error || e.message || 'Помилка'); } finally { setSaving(false); }
    };
    const handleSaveAndRender = async () => {
        if (hasValidationErrors) return;
        setSaving(true); setError('');
        try { await onSave(item.id, draft); await onRender(item.id); } catch (e) { setError(e.response?.data?.error || e.message || 'Помилка'); } finally { setSaving(false); }
    };

    const pickPhoto = () => {
        const inp = document.createElement('input');
        inp.type = 'file'; inp.accept = 'image/*';
        inp.onchange = () => { if (inp.files[0]) { setCropFile(inp.files[0]); setPhotoModal(true); } };
        inp.click();
    };

    const cropOn = (parseInt(draft.crop_width) || 0) > 0 && (parseInt(draft.crop_height) || 0) > 0;
    const frameMode = draft.fit ? 'вмістити 16:9' : cropOn ? `обрізано ${draft.crop_width}×${draft.crop_height}` : 'без обрізання';

    return (
        <div>
            <div className="flex items-center justify-between gap-2 mb-3">
                <h3 className="text-lg font-semibold m-0" style={{ color: '#fff' }}>
                    Питання #{item.position + 1}{dirty && <span style={{ color: 'var(--color-warn)' }}> ●</span>}
                </h3>
                <Button size="sm" variant="danger" onClick={() => onDelete(item.id)}>🗑 Видалити</Button>
            </div>

            {/* Відео */}
            <div className="flex items-center justify-between gap-2 flex-wrap mb-2">
                <span className="text-sm font-medium">Відео {videoChanged && <span style={{ color: 'var(--color-warn)' }}>●</span>}</span>
                <Button size="sm" variant="primary" disabled={!mediaReady} title={!mediaReady ? 'Відео ще завантажується' : ''}
                    onClick={() => setVideoModal(true)}>✂ Обрізати / вмістити + час</Button>
            </div>
            {media?.status === 'error' && (
                <div className="mb-2 flex items-center justify-between gap-2 flex-wrap p-2 rounded-lg"
                    style={{ background: 'rgba(244,96,122,0.12)', border: '1px solid rgba(244,96,122,0.35)' }}>
                    <span className="text-sm" style={{ color: 'var(--color-danger)' }}>⚠️ Не вдалося завантажити відео</span>
                    <Button size="sm" onClick={() => onRetry(item.id)}>↻ Спробувати ще раз</Button>
                </div>
            )}
            {media?.status === 'downloading' && (
                <div className="mb-2 text-sm flex items-center gap-2" style={{ color: 'var(--color-warn)' }}>
                    <Spinner size={14} /> Відео завантажується…
                </div>
            )}
            <div style={{ borderRadius: 12, overflow: 'hidden', border: '1px solid rgba(255,255,255,0.1)',
                maxWidth: 'min(1000px, calc(52vh * 16 / 9))' }}>
                {renderedVideoUrl ? (
                    <video key={renderedVideoUrl} controls src={renderedVideoUrl} style={{ width: '100%', display: 'block', background: '#000', aspectRatio: '16/9' }} />
                ) : embedUrl ? (
                    <iframe key={`yt-${item.id}-${embedStart}`} title="yt" src={embedUrl}
                        style={{ width: '100%', aspectRatio: '16/9', display: 'block', border: 0 }}
                        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowFullScreen />
                ) : (
                    <div style={{ aspectRatio: '16/9', display: 'grid', placeItems: 'center', color: 'var(--color-muted)' }}>Вкажіть YouTube URL</div>
                )}
            </div>
            <div className="mt-2 text-sm flex flex-wrap gap-x-4 gap-y-1" style={{ color: 'var(--color-muted)', maxWidth: 1000 }}>
                <span>Відрізок: <b style={{ color: 'var(--color-text)' }}>{fmtDur(draft.start_time)}–{fmtDur(draft.end_time)}</b> ({Math.max(0, (parseFloat(draft.end_time) || 0) - (parseFloat(draft.start_time) || 0)).toFixed(1)}s)</span>
                <span>Кадр: <b style={{ color: 'var(--color-text)' }}>{frameMode}</b></span>
                {media?.width > 0 && <span>Оригінал: {media.width}×{media.height} · {fmtDur(media.duration)}</span>}
            </div>

            <div className="my-4" style={{ borderTop: '1px solid rgba(255,255,255,0.08)' }} />

            {/* Дві колонки: поля + фото (стек на мобільному) */}
            <div className="grid gap-5 grid-cols-1 sm:grid-cols-[minmax(0,1fr)_220px]">
                <div className="grid gap-3 content-start">
                    <Field label="YouTube URL" changed={isCh('youtube_url')}
                        hint={hasPlaylistParam(draft.youtube_url) ? '⚠️ Збережеться лише відео' : 'Зміна URL запустить нове завантаження'}>
                        <TextInput value={draft.youtube_url} changed={isCh('youtube_url')} onChange={(e) => set('youtube_url', e.target.value)} />
                    </Field>
                    <Field label="Відповідь (текст)" changed={isCh('answer_title')}>
                        <TextInput value={draft.answer_title} changed={isCh('answer_title')} onChange={(e) => set('answer_title', e.target.value)} />
                    </Field>
                    <Field label="Гучність" changed={isCh('volume')}>
                        <div className="flex items-center gap-3">
                            <input type="range" min="0" max="2" step="0.01" value={draft.volume}
                                onChange={(e) => set('volume', parseFloat(e.target.value))} style={{ flex: 1, maxWidth: 240 }} />
                            <span style={{ fontFamily: 'var(--font-mono)', minWidth: 40 }}>{Number(draft.volume).toFixed(2)}</span>
                        </div>
                    </Field>
                    <Toggle checked={draft.show_video} changed={isCh('show_video')} onChange={(v) => set('show_video', v)} label="Показувати відео гравцям" />
                </div>

                <div>
                    <div className="text-sm font-medium mb-2">Фото відповіді {imageStaged && <span style={{ color: 'var(--color-warn)' }}>●</span>}</div>
                    <div style={{ width: '100%', maxWidth: 220, aspectRatio: '1/1', borderRadius: 12, overflow: 'hidden',
                        border: '1px solid rgba(255,255,255,0.12)', background: 'rgba(255,255,255,0.04)', display: 'grid', placeItems: 'center' }}>
                        {displayImg
                            ? <img src={displayImg} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} onError={(e) => { e.target.style.display = 'none'; }} />
                            : <EditorPhotoPlaceholder label={stagedDelete ? 'буде видалено' : 'нема фото'} size={40} />}
                    </div>
                    <div className="mt-2 flex flex-wrap gap-2">
                        <Button size="sm" onClick={pickPhoto}>{displayImg ? '🔄 Замінити' : '+ Фото'}</Button>
                        {displayImg && <Button size="sm" variant="danger" onClick={() => setImage({ _imageFile: null, _imageDelete: true })}>🗑</Button>}
                        {imageStaged && <Button size="sm" variant="ghost" onClick={() => setImage({ _imageFile: null, _imageDelete: false })}>↩</Button>}
                    </div>
                </div>
            </div>

            {error && <p className="mt-3 text-sm" style={{ color: 'var(--color-danger)' }}>{error}</p>}
            {hasValidationErrors && <p className="mt-2 text-xs" style={{ color: 'var(--color-danger)' }}>Перевірте час відрізка у вікні налаштування відео.</p>}

            <div className="mt-4 flex gap-2 flex-wrap">
                <Button variant="primary" onClick={handleSave} disabled={saving || hasValidationErrors || !dirty}>
                    {saving ? <Spinner /> : '💾 Зберегти'}
                </Button>
                <Button onClick={handleSaveAndRender} disabled={saving || hasValidationErrors || !mediaReady}
                    title={!mediaReady ? 'Відео ще завантажується' : 'Зберегти і одразу почати рендер'}>💾▶ Зберегти і рендерити</Button>
                {dirty && <Button variant="ghost" onClick={() => onReset(item.id)}>Скинути зміни</Button>}
            </div>

            <ImageCropModal open={photoModal} file={cropFile}
                onCancel={() => setPhotoModal(false)}
                onConfirm={(blob) => { setImage({ _imageFile: blob, _imageDelete: false }); setPhotoModal(false); }} />
            <VideoCropModal open={videoModal} videoUrl={rawVideoUrl}
                mediaW={media?.width || 0} mediaH={media?.height || 0} duration={media?.duration || 0}
                initial={{ start: parseFloat(draft.start_time) || 0, end: parseFloat(draft.end_time) || 0,
                    cropX: parseInt(draft.crop_x) || 0, cropY: parseInt(draft.crop_y) || 0,
                    cropW: parseInt(draft.crop_width) || 0, cropH: parseInt(draft.crop_height) || 0, fit: !!draft.fit }}
                onCancel={() => setVideoModal(false)}
                onConfirm={(res) => { setMany(res); setVideoModal(false); }} />
        </div>
    );
}

// ─── Форма створення (лише посилання + відповідь) ───────────────────────────
function CreateForm({ pid, cid, onCreate, onCancel, onDirty }) {
    const [form, setForm] = useState({ youtube_url: '', answer_title: '' });
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const set = (k, v) => setForm((p) => ({ ...p, [k]: v }));
    useEffect(() => {
        onDirty(form.youtube_url.trim() !== '' || form.answer_title.trim() !== '');
        return () => onDirty(false);
    }, [form, onDirty]);

    const submit = async (e) => {
        e.preventDefault(); setError(''); setBusy(true);
        try {
            const payload = {
                show_video: false,
                video: { youtube_url: cleanYouTubeUrl(form.youtube_url), start_time: 0, end_time: 15,
                    volume: 1, crop_x: 0, crop_y: 0, crop_width: 0, crop_height: 0, fit: false },
                answer: { title: form.answer_title },
            };
            const r = await api.post(`/api/projects/${pid}/categories/${cid}/items`, payload);
            onCreate(r.data);
        } catch (err) { setError(err.response?.data?.error || 'Помилка створення'); } finally { setBusy(false); }
    };

    return (
        <form onSubmit={submit}>
            <h3 className="text-lg font-semibold m-0 mb-3" style={{ color: '#fff' }}>Нове питання</h3>
            <div className="grid gap-3" style={{ maxWidth: 560 }}>
                <Field label="YouTube URL" hint={hasPlaylistParam(form.youtube_url) ? '⚠️ Збережеться лише відео' : 'Обрізання, час і гучність налаштуєте після завантаження'}>
                    <TextInput autoFocus value={form.youtube_url} placeholder="https://youtube.com/watch?v=..." onChange={(e) => set('youtube_url', e.target.value)} />
                </Field>
                <Field label="Відповідь (текст)">
                    <TextInput value={form.answer_title} placeholder="Напр. Queen — Bohemian Rhapsody" onChange={(e) => set('answer_title', e.target.value)} />
                </Field>
            </div>
            {error && <p className="mt-2 text-sm" style={{ color: 'var(--color-danger)' }}>{error}</p>}
            <div className="mt-4 flex gap-2 flex-wrap">
                <Button type="submit" variant="primary" disabled={busy || !form.youtube_url.trim()}>{busy ? <Spinner /> : 'Створити'}</Button>
                <Button type="button" variant="ghost" onClick={onCancel}>Скасувати</Button>
            </div>
        </form>
    );
}

// ─── Сторінка ───────────────────────────────────────────────────────────────
const pageVariants = {
    initial: { opacity: 0, y: 18 },
    animate: { opacity: 1, y: 0, transition: { duration: 0.22, ease: [0.22, 0.61, 0.36, 1] } },
    exit:    { opacity: 0, y: -10, transition: { duration: 0.16, ease: 'easeIn' } },
};

export default function CategoryDetail() {
    const { pid, cid } = useParams();
    const navigate = useNavigate();

    const [items, setItems] = useState([]); // у порядку відображення (position ASC)
    const [catTitle, setCatTitle] = useState('');
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [isCreating, setIsCreating] = useState(false);
    const [selectedId, setSelectedId] = useState(null);

    // Чернетки, що ЗБЕРІГАЮТЬСЯ між перемиканнями питань: { [id]: form }
    const [drafts, setDrafts] = useState({});
    const [renderVersions, setRenderVersions] = useState({});
    const [imageVersions, setImageVersions] = useState({});
    const [savingAll, setSavingAll] = useState(false);

    const [createDirty, setCreateDirty] = useState(false);

    const itemsRef = useRef(items);
    useEffect(() => { itemsRef.current = items; }, [items]);
    const draftsRef = useRef(drafts);
    useEffect(() => { draftsRef.current = drafts; }, [drafts]);
    const selectedIdRef = useRef(selectedId);
    useEffect(() => { selectedIdRef.current = selectedId; }, [selectedId]);

    // Множина «брудних» питань — обчислюється з чернеток і поточних даних.
    const dirtyIds = useMemo(() => {
        const s = new Set();
        for (const item of items) if (isDraftDirty(item, drafts[item.id])) s.add(item.id);
        return s;
    }, [items, drafts]);
    const pageDirty = dirtyIds.size > 0 || createDirty;
    useUnsavedWarning(pageDirty);

    const guardedNav = (to) => {
        if (pageDirty && !window.confirm('Є незбережені зміни. Вийти без збереження?')) return;
        navigate(to);
    };

    // Перемикання питання — БЕЗ підтвердження: чернетки зберігаються.
    const selectItem = (id) => { setIsCreating(false); setSelectedId(id); };

    const handleWsMessage = useCallback((msg) => {
        const { action, item_id: itemId, data } = msg;
        if (action === 'sync_edit') {
            setDrafts((p) => ({ ...p, [itemId]: { ...(p[itemId] ?? {}), ...data } }));
            return;
        }
        if (action === 'item_render_ready') setRenderVersions((p) => ({ ...p, [itemId]: Date.now() }));
        if (action === 'item_image_updated' || action === 'item_image_deleted') setImageVersions((p) => ({ ...p, [itemId]: Date.now() }));
        if (action === 'item_deleted' && itemId === selectedIdRef.current) setSelectedId(null);
        setItems((prev) => {
            switch (action) {
                case 'item_created': return (!data || prev.some((i) => i.id === data.id)) ? prev : [...prev, data];
                case 'item_updated': return prev.map((i) => i.id === itemId ? data : i);
                case 'item_deleted': return prev.filter((i) => i.id !== itemId);
                case 'item_rendering':
                case 'item_render_ready':
                case 'item_render_error': { const rs = data?.render_status; return prev.map((i) => i.id === itemId ? { ...i, video: { ...i.video, render_status: rs } } : i); }
                case 'item_image_updated': return prev.map((i) => i.id === itemId ? { ...i, answer: { ...i.answer, image_path: data?.image_path } } : i);
                case 'item_image_deleted': return prev.map((i) => i.id === itemId ? { ...i, answer: { ...i.answer, image_path: '' } } : i);
                default: return prev;
            }
        });
    }, []);
    const { wsStatus, sendMessage } = useEditorWS(cid, handleWsMessage);

    const lastSyncTimeRef = useRef(0);
    const sendMessageRef = useRef(sendMessage);
    useEffect(() => { sendMessageRef.current = sendMessage; }, [sendMessage]);

    // Оновлення чернетки конкретного питання (керує дочірнім ItemSettings).
    const updateDraft = useCallback((itemId, patch) => {
        const item = itemsRef.current.find((i) => i.id === itemId);
        const base = draftsRef.current[itemId] ?? (item ? itemToForm(item) : {});
        const next = { ...base, ...patch };
        setDrafts((prev) => ({ ...prev, [itemId]: next }));
        // throttled live-sync (без локального файлу/намірів про фото)
        const now = Date.now();
        if (now - lastSyncTimeRef.current >= SYNC_THROTTLE_MS) {
            lastSyncTimeRef.current = now;
            const rest = { ...next }; delete rest._imageFile; delete rest._imageDelete;
            sendMessageRef.current({ action: 'sync_edit', item_id: itemId, data: rest });
        }
    }, []);

    const resetDraft = useCallback((itemId) => {
        setDrafts((prev) => { const n = { ...prev }; delete n[itemId]; return n; });
    }, []);

    const loadItems = useCallback(async () => {
        try {
            const projRes = await api.get(`/api/projects/${pid}`);
            const cat = (projRes.data.categories || []).find((c) => String(c.id) === String(cid));
            setCatTitle(cat?.title ?? `Категорія #${cid}`);
            const r = await api.get(`/api/projects/${pid}/categories/${cid}/items`);
            setItems(r.data || []);
        } catch (err) {
            setError(err.response?.status === 404 ? 'Категорію не знайдено' : 'Помилка завантаження');
        } finally { setLoading(false); }
    }, [pid, cid]);

    const pollingRef = useRef(null);
    useEffect(() => { loadItems(); return () => clearInterval(pollingRef.current); }, [loadItems]);
    useEffect(() => {
        const hasDownloading = items.some((i) => i.video?.media?.status === 'downloading');
        if (hasDownloading && !pollingRef.current) {
            pollingRef.current = setInterval(async () => {
                try { const r = await api.get(`/api/projects/${pid}/categories/${cid}/items`); setItems(r.data || []); } catch { /* ignore */ }
            }, 4000);
        } else if (!hasDownloading && pollingRef.current) { clearInterval(pollingRef.current); pollingRef.current = null; }
    }, [items, pid, cid]);

    // Збереження одного питання; після успіху чернетка очищується (зникає «брудність»).
    const saveItem = useCallback(async (itemId, draft) => {
        const r = await api.put(`/api/projects/${pid}/categories/${cid}/items/${itemId}`, toPayload(draft));
        let updated = r.data;
        if (draft._imageDelete) {
            await api.delete(`/api/projects/${pid}/categories/${cid}/items/${itemId}/image`);
            updated = { ...updated, answer: { ...updated.answer, image_path: '' } };
        } else if (draft._imageFile) {
            const fd = new FormData(); fd.append('image', draft._imageFile, 'answer.jpg');
            await api.post(`/api/projects/${pid}/categories/${cid}/items/${itemId}/image`, fd, { headers: { 'Content-Type': 'multipart/form-data' } });
            updated = { ...updated, answer: { ...updated.answer, image_path: `/answers/${pid}/${cid}/${itemId}.jpg` } };
        }
        setItems((prev) => prev.map((i) => i.id === itemId ? updated : i));
        if (draft._imageFile || draft._imageDelete) setImageVersions((p) => ({ ...p, [itemId]: Date.now() }));
        setDrafts((prev) => { const n = { ...prev }; delete n[itemId]; return n; });
        return updated;
    }, [pid, cid]);

    // Зберегти ВСІ незбережені питання одразу.
    const handleSaveAll = async () => {
        setSavingAll(true);
        try {
            for (const item of itemsRef.current) {
                const d = draftsRef.current[item.id];
                if (d && isDraftDirty(item, d)) {
                    await saveItem(item.id, d);
                }
            }
        } catch (err) {
            alert(err.response?.data?.error || 'Не вдалося зберегти всі зміни');
        } finally { setSavingAll(false); }
    };

    const handleDelete = async (itemId) => {
        if (!confirm('Видалити питання?')) return;
        await api.delete(`/api/projects/${pid}/categories/${cid}/items/${itemId}`);
        setItems((prev) => prev.filter((i) => i.id !== itemId));
        setDrafts((prev) => { const n = { ...prev }; delete n[itemId]; return n; });
        if (selectedIdRef.current === itemId) setSelectedId(null);
    };

    const handleRender = async (itemId) => {
        try {
            await api.post(`/api/projects/${pid}/categories/${cid}/items/${itemId}/render`);
            setItems((prev) => prev.map((i) => i.id === itemId ? { ...i, video: { ...i.video, render_status: 'rendering' } } : i));
        } catch (err) { alert(err.response?.data?.error || 'Помилка рендеру'); }
    };

    // Перезапуск завантаження для відео, що провалилось (кнопка "спробувати ще раз").
    const handleRetryDownload = async (itemId) => {
        try {
            await api.post(`/api/projects/${pid}/categories/${cid}/items/${itemId}/redownload`);
            // Оптимістично переводимо media у "downloading" → запускається опитування статусу.
            setItems((prev) => prev.map((i) => i.id === itemId
                ? { ...i, video: { ...i.video, media: { ...(i.video?.media || {}), status: 'downloading' } } }
                : i));
        } catch (err) { alert(err.response?.data?.error || 'Не вдалося перезапустити завантаження'); }
    };

    const commitReorder = useCallback(async (id) => {
        const arr = itemsRef.current;
        const newIndex = arr.findIndex((i) => i.id === id);
        const it = arr[newIndex];
        if (!it || newIndex === it.position) return;
        try {
            await api.put(`/api/projects/${pid}/categories/${cid}/items/${id}`, { position: newIndex });
        } catch { /* ignore, перезавантажимо */ }
        try { const r = await api.get(`/api/projects/${pid}/categories/${cid}/items`); setItems(r.data || []); } catch { /* ignore */ }
    }, [pid, cid]);

    if (loading) return <div className="p-6"><Spinner /> <span className="ml-2">Завантаження…</span></div>;
    if (error) return <div className="p-6"><p style={{ color: 'var(--color-danger)' }}>{error}</p><a href={`/projects/${pid}`} onClick={(e) => { e.preventDefault(); navigate(`/projects/${pid}`); }}>← Назад</a></div>;

    const selectedItem = items.find((i) => i.id === selectedId) || null;

    return (
        <motion.div variants={pageVariants} initial="initial" animate="animate" exit="exit">
            <div className="mx-auto max-w-[1600px] px-3 sm:px-4 py-5">
                <div className="flex items-center justify-between gap-2 flex-wrap text-sm" style={{ color: 'var(--color-muted)' }}>
                    <p className="m-0">
                        <a href="/" onClick={(e) => { e.preventDefault(); guardedNav('/'); }}>Проєкти</a>{' / '}
                        <a href={`/projects/${pid}`} onClick={(e) => { e.preventDefault(); guardedNav(`/projects/${pid}`); }}>Проєкт #{pid}</a>{' / '}
                        <span style={{ color: 'var(--color-text)' }}>{catTitle}</span>
                        {pageDirty && <span style={{ color: 'var(--color-warn)', marginLeft: 8 }}>● незбережені зміни</span>}
                    </p>
                    <span title="WebSocket live-sync">{{ connected: '🟢 онлайн', disconnected: '🔴 офлайн', connecting: '🟡 …' }[wsStatus] ?? wsStatus}</span>
                </div>

                <h1 className="mt-2 mb-4 text-2xl sm:text-3xl font-bold" style={{ color: '#fff' }}>{catTitle}</h1>

                <div className="grid gap-4 grid-cols-1 lg:grid-cols-[minmax(360px,480px)_minmax(0,1fr)]">
                    {/* ЛІВОРУЧ: список */}
                    <div>
                        <div className="flex flex-wrap items-center gap-2 mb-3">
                            <Button variant="primary" size="sm" onClick={() => { setIsCreating(true); setSelectedId(null); }}>+ Нове питання</Button>
                        </div>
                        <p className="text-xs mb-2" style={{ color: 'var(--color-muted)' }}>{items.length} питань · перетягуйте ⠿ для порядку</p>
                        {items.length === 0 ? (
                            <p style={{ color: 'var(--color-muted)' }}>Питань ще немає.</p>
                        ) : (
                            <Reorder.Group as="div" axis="y" values={items} onReorder={setItems}>
                                {items.map((item, idx) => (
                                    <ItemRow key={item.id} item={item} index={idx}
                                        selected={selectedId === item.id} dirty={dirtyIds.has(item.id)}
                                        onSelect={selectItem} onRender={handleRender} onRetry={handleRetryDownload} onCommit={commitReorder} />
                                ))}
                            </Reorder.Group>
                        )}
                    </div>

                    {/* ПРАВОРУЧ: налаштування */}
                    <div className="self-start">
                        <div className="mb-3 flex">
                            <Button variant="primary" onClick={handleSaveAll} disabled={savingAll || dirtyIds.size === 0}
                                title="Зберегти всі незбережені питання категорії">
                                {savingAll ? <Spinner /> : `💾 Зберегти всі${dirtyIds.size ? ` (${dirtyIds.size})` : ''}`}
                            </Button>
                        </div>
                        <Glass className="p-4 sm:p-5" style={{ minHeight: 320 }}>
                        <AnimatePresence mode="wait">
                            {isCreating ? (
                                <motion.div key="create" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
                                    <CreateForm pid={pid} cid={cid} onDirty={setCreateDirty}
                                        onCreate={(item) => { setItems((p) => p.some((i) => i.id === item.id) ? p : [...p, item]); setIsCreating(false); setSelectedId(item.id); }}
                                        onCancel={() => setIsCreating(false)} />
                                </motion.div>
                            ) : selectedItem ? (
                                <motion.div key={selectedItem.id} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
                                    <ItemSettings item={selectedItem} pid={pid} cid={cid}
                                        draft={drafts[selectedItem.id] ?? itemToForm(selectedItem)}
                                        onChange={(patch) => updateDraft(selectedItem.id, patch)}
                                        renderVersion={renderVersions[selectedItem.id] ?? 0}
                                        imageVersion={imageVersions[selectedItem.id] ?? 0}
                                        onSave={saveItem} onRender={handleRender} onDelete={handleDelete} onReset={resetDraft} onRetry={handleRetryDownload} />
                                </motion.div>
                            ) : (
                                <motion.div key="empty" initial={{ opacity: 0 }} animate={{ opacity: 1 }}
                                    className="grid place-items-center text-center" style={{ minHeight: 280, color: 'var(--color-muted)' }}>
                                    <div>
                                        <div style={{ fontSize: 40 }}>🎵</div>
                                        <p className="mt-2">Оберіть питання зліва<br />або створіть нове.</p>
                                    </div>
                                </motion.div>
                            )}
                        </AnimatePresence>
                        </Glass>
                    </div>
                </div>
            </div>
        </motion.div>
    );
}
