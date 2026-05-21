import { useEffect, useState, useRef, useCallback } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import api from '../api';

// Попереджає про незбережені зміни при закритті/перезавантаженні вкладки.
function useUnsavedWarning(dirty) {
    useEffect(() => {
        if (!dirty) return;
        const handler = (e) => { e.preventDefault(); e.returnValue = ''; };
        window.addEventListener('beforeunload', handler);
        return () => window.removeEventListener('beforeunload', handler);
    }, [dirty]);
}

// ─── Constants ────────────────────────────────────────────────────────────────

// Відносні до поточного origin: працює і коли фронтенд роздає Go (:8080),
// і у dev через Vite-proxy. API_BASE порожній → шляхи типу `/media/...`.
const API_BASE = '';
const WS_BASE  = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}`;
const SYNC_THROTTLE_MS = 50;

// ─── Helpers ──────────────────────────────────────────────────────────────────

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

function formatDuration(secs) {
    if (!secs) return '—';
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${String(s).padStart(2, '0')} (${Math.round(secs)}s)`;
}

// ─── Crop validation ──────────────────────────────────────────────────────────

/**
 * Повертає об'єкт { field: 'повідомлення помилки' }.
 * Порожній об'єкт = всі поля коректні.
 * `media` — об'єкт з полями width, height, duration (можуть бути 0 якщо ще не завантажено).
 */
function validateCropForm(form, media) {
    const errors = {};
    const start = parseFloat(form.start_time) || 0;
    const end   = parseFloat(form.end_time)   || 0;
    const cx    = parseInt(form.crop_x)        || 0;
    const cy    = parseInt(form.crop_y)        || 0;
    const cw    = parseInt(form.crop_width)    || 0;
    const ch    = parseInt(form.crop_height)   || 0;

    // Часові мітки — базова перевірка
    if (start < 0)
        errors.start_time = 'Не може бути від\'ємним';
    if (end > 0 && end <= start)
        errors.end_time = `Має бути > ${start.toFixed(1)}s`;

    // Часові мітки — відносно тривалості відео
    const dur = media?.duration ?? 0;
    if (dur > 0) {
        if (!errors.start_time && start >= dur)
            errors.start_time = `${start.toFixed(1)}s ≥ тривалість (${dur.toFixed(1)}s)`;
        if (!errors.end_time && end > dur)
            errors.end_time = `${end.toFixed(1)}s > тривалість (${dur.toFixed(1)}s)`;
    }

    // Crop — тільки якщо задано хоч одне ненульове значення
    if (cw > 0 || ch > 0) {
        if (cx < 0) errors.crop_x = '< 0';
        if (cy < 0) errors.crop_y = '< 0';
        if (cw <= 0) errors.crop_width  = 'Має бути > 0';
        if (ch <= 0) errors.crop_height = 'Має бути > 0';

        const mw = media?.width  ?? 0;
        const mh = media?.height ?? 0;
        if (mw > 0 && !errors.crop_width  && cx + cw > mw)
            errors.crop_width  = `X(${cx})+W(${cw})=${cx+cw} > ширина відео(${mw})`;
        if (mh > 0 && !errors.crop_height && cy + ch > mh)
            errors.crop_height = `Y(${cy})+H(${ch})=${cy+ch} > висота відео(${mh})`;
    }

    return errors;
}

// ─── Form helpers ─────────────────────────────────────────────────────────────

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
        answer_title: item.answer?.title ?? '',
    };
}
const EMPTY_FORM = {
    show_video: false, youtube_url: '', start_time: 0, end_time: 30,
    volume: 1, crop_x: 0, crop_y: 0, crop_width: 0, crop_height: 0, answer_title: '',
};
function toPayload(f) {
    return {
        show_video: f.show_video,
        video: {
            youtube_url: cleanYouTubeUrl(f.youtube_url),
            start_time:  parseFloat(f.start_time)  || 0,
            end_time:    parseFloat(f.end_time)     || 0,
            volume:      parseFloat(f.volume)       || 1,
            crop_x:      parseInt(f.crop_x)         || 0,
            crop_y:      parseInt(f.crop_y)         || 0,
            crop_width:  parseInt(f.crop_width)     || 0,
            crop_height: parseInt(f.crop_height)    || 0,
        },
        answer: { title: f.answer_title },
    };
}

// ─── Badges ───────────────────────────────────────────────────────────────────

function renderBadge(s) {
    return { ready: '✅ готовий', rendering: '⏳ рендер...', unrendered: '⬜ не рендеровано', error: '❌ помилка' }[s] ?? s ?? '—';
}
function mediaBadge(s) {
    return { ready: '✅ завантажено', downloading: '⏳ завантаж...', error: '❌ помилка' }[s] ?? s ?? '—';
}

// ─── WebSocket hook ───────────────────────────────────────────────────────────

function useEditorWS(cid, onMessage) {
    const [wsStatus, setWsStatus] = useState('connecting');
    const wsRef        = useRef(null);
    const onMessageRef = useRef(onMessage);
    useEffect(() => { onMessageRef.current = onMessage; }, [onMessage]);

    useEffect(() => {
        if (!cid) return;
        let alive = true;
        let reconnectTimer = null;

        function connect() {
            if (!alive) return;
            const token = localStorage.getItem('token');
            const ws = new WebSocket(`${WS_BASE}/api/categories/${cid}/ws?token=${token}`);
            wsRef.current = ws;
            setWsStatus('connecting');
            ws.onopen    = () => { if (alive) setWsStatus('connected'); };
            ws.onerror   = () => {};
            ws.onclose   = () => {
                if (!alive) return;
                setWsStatus('disconnected');
                reconnectTimer = setTimeout(connect, 3000);
            };
            ws.onmessage = (e) => {
                try { onMessageRef.current(JSON.parse(e.data)); } catch {}
            };
        }

        connect();
        return () => {
            alive = false;
            clearTimeout(reconnectTimer);
            wsRef.current?.close();
        };
    }, [cid]);

    const sendMessage = useCallback((msg) => {
        const ws = wsRef.current;
        if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
    }, []);

    return { wsStatus, sendMessage };
}

// ─── Inline error display ─────────────────────────────────────────────────────

function FieldError({ msg }) {
    if (!msg) return null;
    return <small style={{ color: 'red', display: 'block', marginTop: '2px' }}>{msg}</small>;
}

function errBorder(msg) {
    return msg ? { border: '1px solid red' } : {};
}

// ─── VideoFields ──────────────────────────────────────────────────────────────

/**
 * mediaInfo = item.video?.media (з полями width, height, duration після завантаження)
 * Рендерить поля форми з inline-валідацією та інформацією про відео.
 */
function VideoFields({ form, set, mediaInfo }) {
    const errors    = validateCropForm(form, mediaInfo);
    const hasErrors = Object.keys(errors).length > 0;
    const mw = mediaInfo?.width    ?? 0;
    const mh = mediaInfo?.height   ?? 0;
    const md = mediaInfo?.duration ?? 0;

    return (
        <>
            {/* Інформація про відео-файл */}
            {mediaInfo?.status === 'ready' && (
                <div style={{ marginBottom: '10px', fontSize: '0.85em', color: '#555', background: '#f5f5f5', padding: '6px 10px', display: 'inline-block' }}>
                    📐 {mw > 0 ? `${mw}×${mh}` : '? ×?'} &nbsp;
                    ⏱ {md > 0 ? formatDuration(md) : '?'} &nbsp;
                    {mw === 0 && <em style={{ color: '#aaa' }}>(метадані ще завантажуються)</em>}
                </div>
            )}

            <table>
                <tbody>
                    {/* YouTube URL */}
                    <tr>
                        <td style={tdL}>YouTube URL:</td>
                        <td style={tdR}>
                            <input style={{ width: '420px' }} value={form.youtube_url}
                                onChange={e => set('youtube_url', e.target.value)}
                                placeholder="https://youtube.com/watch?v=..." />
                            {hasPlaylistParam(form.youtube_url) && (
                                <small style={{ color: '#c66', display: 'block', marginTop: '2px' }}>
                                    ⚠️ URL містить плейлист — збережеться тільки відео
                                </small>
                            )}
                        </td>
                    </tr>

                    {/* Відповідь */}
                    <tr>
                        <td style={tdL}>Відповідь:</td>
                        <td style={tdR}>
                            <input style={{ width: '420px' }} value={form.answer_title}
                                onChange={e => set('answer_title', e.target.value)} placeholder="Текст відповіді" />
                        </td>
                    </tr>

                    {/* Показ відео */}
                    <tr>
                        <td style={tdL}>Показ відео:</td>
                        <td style={tdR}>
                            <input type="checkbox" checked={form.show_video}
                                onChange={e => set('show_video', e.target.checked)} />
                            <label style={{ marginLeft: '6px' }}>
                                {form.show_video ? 'відео показується гравцям' : 'відео приховано'}
                            </label>
                        </td>
                    </tr>

                    {/* Часові мітки */}
                    <tr>
                        <td style={tdL}>Часові мітки (сек):</td>
                        <td style={tdR}>
                            <label>Початок:</label>{' '}
                            <input type="number" step="0.1" min="0" style={{ width: '80px', ...errBorder(errors.start_time) }}
                                value={form.start_time} onChange={e => set('start_time', e.target.value)} />
                            {' — '}
                            <label>Кінець:</label>{' '}
                            <input type="number" step="0.1" min="0" style={{ width: '80px', ...errBorder(errors.end_time) }}
                                value={form.end_time} onChange={e => set('end_time', e.target.value)} />
                            {' '}
                            <small style={{ color: '#888' }}>
                                ({Math.max(0, (parseFloat(form.end_time)||0) - (parseFloat(form.start_time)||0)).toFixed(1)}s)
                            </small>
                            <FieldError msg={errors.start_time} />
                            <FieldError msg={errors.end_time} />
                        </td>
                    </tr>

                    {/* Гучність */}
                    <tr>
                        <td style={tdL}>Гучність:</td>
                        <td style={tdR}>
                            {/* FIX: фіксована ширина → слайдер не стрибає при зміні тексту */}
                            <span style={{ display: 'inline-block', width: '44px', fontSize: '0.82em', color: '#888', verticalAlign: 'middle' }}>
                                {form.volume < 0.5 ? 'тихо' : form.volume > 1.5 ? 'гучно' : ''}
                            </span>
                            <input type="range" min="0" max="2" step="0.01"
                                style={{ width: '180px', verticalAlign: 'middle' }}
                                value={form.volume}
                                onChange={e => set('volume', parseFloat(e.target.value))} />
                            {' '}
                            <input type="number" min="0" max="2" step="0.01" style={{ width: '60px' }}
                                value={Number(form.volume).toFixed(2)}
                                onChange={e => set('volume', parseFloat(e.target.value) || 0)} />
                        </td>
                    </tr>

                    {/* Crop */}
                    <tr>
                        <td style={tdL}>Crop (обрізання):</td>
                        <td style={tdR}>
                            <table style={{ borderCollapse: 'collapse', display: 'inline-block' }}>
                                <tbody>
                                    <tr>
                                        <td style={{ paddingRight: '4px' }}>X:</td>
                                        <td>
                                            <input type="number" value={form.crop_x} style={{ width: '68px', ...errBorder(errors.crop_x) }}
                                                onChange={e => set('crop_x', e.target.value)} />
                                        </td>
                                        <td style={{ paddingLeft: '8px', paddingRight: '4px' }}>Y:</td>
                                        <td>
                                            <input type="number" value={form.crop_y} style={{ width: '68px', ...errBorder(errors.crop_y) }}
                                                onChange={e => set('crop_y', e.target.value)} />
                                        </td>
                                    </tr>
                                    <tr>
                                        <td>W:</td>
                                        <td>
                                            <input type="number" value={form.crop_width} style={{ width: '68px', ...errBorder(errors.crop_width) }}
                                                onChange={e => set('crop_width', e.target.value)} />
                                        </td>
                                        <td style={{ paddingLeft: '8px' }}>H:</td>
                                        <td>
                                            <input type="number" value={form.crop_height} style={{ width: '68px', ...errBorder(errors.crop_height) }}
                                                onChange={e => set('crop_height', e.target.value)} />
                                        </td>
                                    </tr>
                                </tbody>
                            </table>
                            {' '}
                            <small style={{ color: '#aaa' }}>
                                {(parseInt(form.crop_width)>0 && parseInt(form.crop_height)>0)
                                    ? `crop=${form.crop_width}:${form.crop_height}:${form.crop_x}:${form.crop_y}`
                                    : 'без обрізання (W=0 або H=0)'}
                                {mw > 0 && ` | відео: ${mw}×${mh}`}
                            </small>
                            {/* Inline помилки crop */}
                            {(errors.crop_x || errors.crop_y) && (
                                <FieldError msg={[errors.crop_x && `X: ${errors.crop_x}`, errors.crop_y && `Y: ${errors.crop_y}`].filter(Boolean).join(' | ')} />
                            )}
                            {(errors.crop_width || errors.crop_height) && (
                                <FieldError msg={[errors.crop_width, errors.crop_height].filter(Boolean).join(' | ')} />
                            )}
                        </td>
                    </tr>
                </tbody>
            </table>

            {hasErrors && (
                <p style={{ color: 'red', fontSize: '0.85em', marginTop: '6px' }}>
                    ⚠️ Є помилки валідації. Виправте їх перед збереженням.
                </p>
            )}
        </>
    );
}

const tdL = { paddingRight: '12px', paddingBottom: '8px', whiteSpace: 'nowrap', verticalAlign: 'top', paddingTop: '4px' };
const tdR = { paddingBottom: '8px' };

// ─── CreateForm ───────────────────────────────────────────────────────────────

function CreateForm({ pid, cid, onCreate, onCancel, onDirty }) {
    const [form, setForm] = useState(EMPTY_FORM);
    const [error, setError] = useState('');
    const set = (k, v) => setForm(p => ({ ...p, [k]: v }));

    // Звітуємо сторінці про незбережений чернетковий вміст форми створення.
    useEffect(() => {
        onDirty(JSON.stringify(form) !== JSON.stringify(EMPTY_FORM));
        return () => onDirty(false);
    }, [form, onDirty]);

    const cropErrors = validateCropForm(form, null);
    const hasErrors  = Object.keys(cropErrors).length > 0;

    const handleSubmit = async (e) => {
        e.preventDefault();
        if (hasErrors) return;
        setError('');
        try {
            const r = await api.post(`/api/projects/${pid}/categories/${cid}/items`, toPayload(form));
            onCreate(r.data);
            setForm(EMPTY_FORM);
        } catch (err) {
            setError(err.response?.data?.error || 'Помилка створення');
        }
    };

    return (
        <form onSubmit={handleSubmit}
            style={{ border: '2px solid #555', padding: '12px', margin: '8px 0', background: '#f9f9f9' }}>
            <strong>Нове питання</strong><br /><br />
            <VideoFields form={form} set={set} mediaInfo={null} />
            {error && <p style={{ color: 'red' }}>{error}</p>}
            <button type="submit" disabled={hasErrors}>Створити</button>{' '}
            <button type="button" onClick={onCancel}>Скасувати</button>
        </form>
    );
}

// ─── AnswerImage ──────────────────────────────────────────────────────────────

/**
 * Презентаційне фото відповіді з підтримкою чорнових (незбережених) змін.
 * url — що показати (прев'ю файлу, серверне фото з версією, або null = плейсхолдер).
 * staged — чи є незбережена зміна фото (новий файл або позначка видалення).
 */
function AnswerImage({ url, staged, size = 200, onPick, onMarkDelete, onUndo }) {
    const btn = { fontSize: '0.8em' };
    return (
        <div style={{ display: 'inline-block' }}>
            {url ? (
                <img src={url} alt="answer"
                    style={{ width: size, height: size, objectFit: 'cover', border: '1px solid #ccc', display: 'block' }}
                    onError={e => { e.target.style.display = 'none'; }} />
            ) : (
                <div style={{ width: size, height: size, border: '1px dashed #bbb',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    color: '#aaa', fontSize: '0.85em', textAlign: 'center' }}>
                    {staged ? 'Фото буде видалено' : 'Немає фото'}
                </div>
            )}
            <div style={{ marginTop: 4, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                <button type="button" style={btn} onClick={onPick}>{url ? '🔄 Замінити' : '+ Фото'}</button>
                {url && <button type="button" style={{ ...btn, color: 'red' }} onClick={onMarkDelete}>🗑 Видалити</button>}
                {staged && <button type="button" style={btn} onClick={onUndo}>↩ Скасувати зміну</button>}
            </div>
            {staged && <small style={{ color: '#c80', display: 'block', marginTop: 2 }}>● незбережена зміна фото</small>}
        </div>
    );
}

// ─── ItemCard ─────────────────────────────────────────────────────────────────

function ItemCard({
    item, idx, total, pid, cid,
    isExpanded, liveEdit, renderVersion, imageVersion,
    onOpen, onClose, onSaveStart, onSaveEnd, onDraftChange, onDirtyChange,
    onSave, onDelete, onRender, onMove, onQuickToggleShowVideo,
}) {
    const [draft, setDraft] = useState(null);
    const [saving, setSaving] = useState(false);
    const [error, setError]   = useState('');
    const [filePreview, setFilePreview] = useState(null);

    const token = localStorage.getItem('token');

    // Cache-busting: renderVersion/imageVersion змінюються при оновленні через WS/збереженні
    const renderedVideoUrl = item.video?.render_status === 'ready'
        ? `${API_BASE}/media/${pid}/${cid}/${item.id}.mp4?token=${token}&v=${renderVersion}`
        : null;

    const ytId       = extractYouTubeId(item.video?.youtube_url);
    const embedStart = draft ? Math.floor(parseFloat(draft.start_time) || 0) : Math.floor(item.video?.start_time || 0);
    const embedUrl   = ytId ? `https://www.youtube.com/embed/${ytId}?start=${embedStart}&rel=0` : null;

    // Серверне фото з версією (cache-bust) + чорновий стейджинг (файл / видалення)
    const serverImg = item.answer?.image_path
        ? `${API_BASE}${item.answer.image_path}?token=${token}&v=${imageVersion}`
        : null;
    const stagedFile   = draft?._imageFile || null;
    const stagedDelete = !!draft?._imageDelete;
    const imageStaged  = Boolean(stagedFile || stagedDelete);
    const displayImg   = stagedDelete ? null : (filePreview || serverImg);

    // Thumbnail для collapsed row (серверне фото з версією)
    const thumbUrl = serverImg;

    const validationErrors = draft ? validateCropForm(draft, item.video?.media) : {};
    const hasValidationErrors = Object.keys(validationErrors).length > 0;

    // Прев'ю для чорнового файлу
    useEffect(() => {
        if (!stagedFile) { setFilePreview(null); return; }
        const u = URL.createObjectURL(stagedFile);
        setFilePreview(u);
        return () => URL.revokeObjectURL(u);
    }, [stagedFile]);

    // При зміні liveEdit (remote sync_edit) — мержимо в draft
    useEffect(() => {
        if (!isExpanded || !liveEdit || !draft) return;
        setDraft(prev => ({ ...prev, ...liveEdit }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [liveEdit]);

    // Чи є незбережені зміни (поля або фото) — звітуємо сторінці
    const dirty = (() => {
        if (!draft) return false;
        if (imageStaged) return true;
        const base = itemToForm(item);
        return Object.keys(base).some(k => String(base[k]) !== String(draft[k]));
    })();
    useEffect(() => { onDirtyChange(item.id, dirty); }, [dirty, item.id, onDirtyChange]);
    useEffect(() => () => onDirtyChange(item.id, false), [item.id, onDirtyChange]);

    const set = useCallback((k, v) => {
        setDraft(prev => {
            const next = { ...prev, [k]: v };
            onDraftChange(item.id, next);
            return next;
        });
    }, [item.id, onDraftChange]);

    // Зміна фото лише локально (без WS-синку — файл не серіалізується в JSON)
    const setImage = (patch) => setDraft(prev => ({ ...prev, ...patch }));

    const handleOpen = () => {
        const base = itemToForm(item);
        setDraft(liveEdit ? { ...base, ...liveEdit } : base);
        setError('');
        onOpen(item.id);
    };
    const handleClose = () => { setDraft(null); onClose(); setError(''); };

    // Зберігає поля + застосовує стейджинг фото; лишає меню розгорнутим.
    const persist = async () => {
        const updated = await onSave(item.id, draft);
        setDraft(itemToForm(updated)); // скидає чорновик/стейджинг, але НЕ згортає
        return updated;
    };

    const handleSave = async () => {
        if (hasValidationErrors) return;
        setSaving(true); setError(''); onSaveStart(item.id);
        try { await persist(); }
        catch (e) { setError(e.response?.data?.error || e.message || 'Помилка збереження'); }
        finally { setSaving(false); onSaveEnd(); }
    };

    const handleSaveAndRender = async () => {
        if (hasValidationErrors) return;
        setSaving(true); setError(''); onSaveStart(item.id);
        try {
            await persist();
            await onRender(item.id);
        } catch (e) {
            setError(e.response?.data?.error || e.message || 'Помилка');
        } finally {
            setSaving(false); onSaveEnd();
        }
    };

    const handlePickImage = () => {
        const inp = document.createElement('input');
        inp.type = 'file'; inp.accept = 'image/*';
        inp.onchange = () => { if (inp.files[0]) setImage({ _imageFile: inp.files[0], _imageDelete: false }); };
        inp.click();
    };
    const handleMarkDelete = () => setImage({ _imageFile: null, _imageDelete: true });
    const handleUndoImage  = () => setImage({ _imageFile: null, _imageDelete: false });

    const displayVolume = isExpanded
        ? (draft?.volume ?? item.video?.volume ?? 1)
        : (liveEdit?.volume ?? item.video?.volume ?? 1);
    const isLive = Boolean(liveEdit);
    const mediaReady = item.video?.media?.status === 'ready';

    return (
        <div style={{ border: '1px solid #bbb', margin: '10px 0' }}>

            {/* ── Summary row ─────────────────────────────────────── */}
            <div style={{
                padding: '8px 12px', display: 'flex', alignItems: 'center',
                gap: '10px', flexWrap: 'wrap',
                background: isExpanded ? '#f0f0f0' : 'transparent',
            }}>
                <span style={{ minWidth: '28px', color: '#888', fontSize: '0.82em' }}>#{item.position + 1}</span>

                <span>
                    <button onClick={() => onMove(item, 'up')}   disabled={idx === 0}          title="Вгору">↑</button>
                    <button onClick={() => onMove(item, 'down')}  disabled={idx === total - 1}  title="Вниз">↓</button>
                </span>

                {/* Thumbnail фото відповіді */}
                {thumbUrl && (
                    <img src={thumbUrl} alt="answer"
                        style={{ width: 36, height: 36, objectFit: 'cover', border: '1px solid #ccc', borderRadius: '2px' }}
                        onError={e => { e.target.style.display = 'none'; }}
                    />
                )}

                <strong style={{ minWidth: '100px' }}>
                    {item.answer?.title || <em style={{ color: '#aaa', fontWeight: 'normal' }}>без відповіді</em>}
                </strong>

                {/* Live-гучність */}
                <span style={{ fontSize: '0.82em', color: isLive && !isExpanded ? '#06a' : '#888', fontFamily: 'monospace' }}>
                    🔊 {Number(displayVolume).toFixed(2)}{isLive && !isExpanded && ' ✏️'}
                </span>

                <label title="Показ відео у грі" style={{ cursor: 'pointer', userSelect: 'none', fontSize: '0.9em' }}>
                    <input type="checkbox" checked={item.show_video}
                        onChange={e => onQuickToggleShowVideo(item.id, e.target.checked)}
                        style={{ marginRight: '4px' }} />
                    відео
                </label>

                <span style={{ fontSize: '0.82em', color: '#555' }}>{mediaBadge(item.video?.media?.status)}</span>
                <span style={{ fontSize: '0.82em', color: '#555' }}>{renderBadge(item.video?.render_status)}</span>

                <button onClick={isExpanded ? handleClose : handleOpen}>
                    {isExpanded ? '▲ Згорнути' : '▼ Розгорнути'}
                </button>

                {mediaReady && item.video?.render_status !== 'rendering' && (
                    <button onClick={() => onRender(item.id)}>▶ Рендер</button>
                )}

                <button onClick={() => onDelete(item.id)} style={{ color: 'red' }}>🗑</button>
            </div>

            {/* ── Expanded panel ──────────────────────────────────── */}
            {isExpanded && draft && (
                <div style={{ padding: '12px', borderTop: '1px solid #bbb' }}>

                    {/* Відео + фото поряд */}
                    <div style={{ display: 'flex', gap: '24px', marginBottom: '16px', flexWrap: 'wrap', alignItems: 'flex-start' }}>

                        {/* Оригінальне YouTube відео */}
                        <div>
                            <p style={{ margin: '0 0 4px' }}>
                                <strong>Оригінал</strong>{' '}
                                <small style={{ color: '#888' }}>з {Number(draft.start_time).toFixed(1)}s</small>
                            </p>
                            {embedUrl ? (
                                <iframe key={`yt-${item.id}-${embedStart}`}
                                    width="480" height="270" src={embedUrl} title="YouTube"
                                    frameBorder="0"
                                    allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
                                    allowFullScreen />
                            ) : (
                                <Placeholder w={480} h={270} text="Вкажіть YouTube URL" />
                            )}
                        </div>

                        {/* Відрендерований кліп */}
                        <div>
                            <p style={{ margin: '0 0 4px' }}><strong>Відрендерований кліп</strong></p>
                            {renderedVideoUrl ? (
                                <video key={renderedVideoUrl} width="480" height="270" controls
                                    style={{ background: '#000', display: 'block' }}
                                    src={renderedVideoUrl} />
                            ) : (
                                <Placeholder w={480} h={270}
                                    text={item.video?.render_status === 'rendering' ? '⏳ рендериться...' : 'ще не відрендеровано'} />
                            )}
                        </div>

                        {/* Фото відповіді */}
                        <div>
                            <p style={{ margin: '0 0 4px' }}><strong>Фото відповіді</strong></p>
                            <AnswerImage
                                url={displayImg} staged={imageStaged} size={200}
                                onPick={handlePickImage}
                                onMarkDelete={handleMarkDelete}
                                onUndo={handleUndoImage}
                            />
                        </div>
                    </div>

                    {/* Поля редагування */}
                    <VideoFields form={draft} set={set} mediaInfo={item.video?.media} />

                    {error && <p style={{ color: 'red' }}>{error}</p>}
                    <div style={{ marginTop: '10px' }}>
                        <button onClick={handleSave} disabled={saving || hasValidationErrors}
                            title={hasValidationErrors ? 'Виправте помилки валідації' : ''}>
                            {saving ? 'Збереження...' : 'Зберегти зміни'}
                        </button>{' '}
                        <button onClick={handleSaveAndRender} disabled={saving || hasValidationErrors || !mediaReady}
                            title={!mediaReady ? 'Відео ще завантажується' : 'Зберегти і одразу почати рендер'}>
                            💾▶ Зберегти і рендерити
                        </button>{' '}
                        <button onClick={handleClose}>Скасувати</button>
                        {hasValidationErrors && (
                            <small style={{ color: 'red', marginLeft: '10px' }}>
                                Збереження заблоковано: є помилки валідації ↑
                            </small>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
}

function Placeholder({ w, h, text }) {
    return (
        <div style={{ width: w, height: h, border: '1px solid #ddd', background: '#fafafa',
            display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#aaa' }}>
            {text}
        </div>
    );
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function CategoryDetail() {
    const { pid, cid } = useParams();

    const [items, setItems]           = useState([]);
    const [catTitle, setCatTitle]     = useState('');
    const [loading, setLoading]       = useState(true);
    const [error, setError]           = useState('');
    const [isCreating, setIsCreating] = useState(false);

    const [expandedId, setExpandedId]      = useState(null);
    const [pendingSaveId, setPendingSaveId] = useState(null);
    const expandedIdRef    = useRef(null);
    const pendingSaveIdRef = useRef(null);
    useEffect(() => { expandedIdRef.current = expandedId; },     [expandedId]);
    useEffect(() => { pendingSaveIdRef.current = pendingSaveId; }, [pendingSaveId]);

    const [liveEdits, setLiveEdits]         = useState({});
    const [renderVersions, setRenderVersions] = useState({});
    const [imageVersions, setImageVersions] = useState({});

    // Незбережені зміни: множина id питань з відкритими чернетками + форма створення.
    const [dirtyItems, setDirtyItems]   = useState(() => new Set());
    const [createDirty, setCreateDirty] = useState(false);
    const pageDirty = dirtyItems.size > 0 || createDirty;
    useUnsavedWarning(pageDirty);

    const navigate = useNavigate();
    const guardedNav = (to) => {
        if (pageDirty && !window.confirm('Є незбережені зміни. Вийти без збереження?')) return;
        navigate(to);
    };

    const handleDirtyChange = useCallback((itemId, isDirty) => {
        setDirtyItems(prev => {
            const has = prev.has(itemId);
            if (isDirty === has) return prev;
            const n = new Set(prev);
            if (isDirty) n.add(itemId); else n.delete(itemId);
            return n;
        });
    }, []);

    const pollingRef = useRef(null);

    // ── WS message handler ────────────────────────────────────────────────────

    const handleWsMessage = useCallback((msg) => {
        const { action, item_id: itemId, data } = msg;

        if (action === 'sync_edit') {
            setLiveEdits(prev => ({ ...prev, [itemId]: data }));
            return;
        }

        if (action === 'item_render_ready') {
            setRenderVersions(prev => ({ ...prev, [itemId]: Date.now() }));
        }

        // Фото змінилось деінде — оновлюємо версію, щоб збити кеш зображення
        if (action === 'item_image_updated' || action === 'item_image_deleted') {
            setImageVersions(prev => ({ ...prev, [itemId]: Date.now() }));
        }

        if (action === 'item_deleted' && itemId === expandedIdRef.current) {
            setExpandedId(null);
        }

        setItems(prev => {
            switch (action) {
                case 'item_created':
                    return prev.some(i => i.id === itemId) ? prev : [...prev, data];
                case 'item_updated':
                    return prev.map(i => i.id === itemId ? data : i);
                case 'item_deleted':
                    return prev.filter(i => i.id !== itemId);
                case 'item_rendering':
                case 'item_render_ready':
                case 'item_render_error': {
                    const rs = data?.render_status;
                    return prev.map(i => i.id === itemId ? { ...i, video: { ...i.video, render_status: rs } } : i);
                }
                case 'item_image_updated':
                    return prev.map(i => i.id === itemId
                        ? { ...i, answer: { ...i.answer, image_path: data?.image_path } } : i);
                case 'item_image_deleted':
                    return prev.map(i => i.id === itemId
                        ? { ...i, answer: { ...i.answer, image_path: '' } } : i);
                default:
                    return prev;
            }
        });
    }, []);

    const { wsStatus, sendMessage } = useEditorWS(cid, handleWsMessage);

    // ── Throttled sync_edit sender ────────────────────────────────────────────

    const lastSyncTimeRef = useRef(0);
    const sendMessageRef  = useRef(sendMessage);
    useEffect(() => { sendMessageRef.current = sendMessage; }, [sendMessage]);

    const handleDraftChange = useCallback((itemId, newDraft) => {
        const now = Date.now();
        if (now - lastSyncTimeRef.current < SYNC_THROTTLE_MS) return;
        lastSyncTimeRef.current = now;
        sendMessageRef.current({ action: 'sync_edit', item_id: itemId, data: newDraft });
    }, []);

    // ── Load ──────────────────────────────────────────────────────────────────

    useEffect(() => {
        loadItems();
        return () => clearInterval(pollingRef.current);
    }, [pid, cid]);

    useEffect(() => {
        const hasDownloading = items.some(i => i.video?.media?.status === 'downloading');
        if (hasDownloading && !pollingRef.current) {
            pollingRef.current = setInterval(async () => {
                try {
                    const r = await api.get(`/api/projects/${pid}/categories/${cid}/items`);
                    setItems(r.data || []);
                } catch {}
            }, 4000);
        } else if (!hasDownloading && pollingRef.current) {
            clearInterval(pollingRef.current);
            pollingRef.current = null;
        }
    }, [items]);

    const loadItems = async () => {
        try {
            const projRes = await api.get(`/api/projects/${pid}`);
            const cat = (projRes.data.categories || []).find(c => String(c.id) === String(cid));
            setCatTitle(cat?.title ?? `Категорія #${cid}`);
            const r = await api.get(`/api/projects/${pid}/categories/${cid}/items`);
            setItems(r.data || []);
        } catch (err) {
            if (err.response?.status === 404) setError('Категорію не знайдено');
            else setError('Помилка завантаження');
        } finally {
            setLoading(false);
        }
    };

    // ── Callbacks ─────────────────────────────────────────────────────────────

    const handleOpen      = useCallback((id) => setExpandedId(id), []);
    const handleClose     = useCallback(() => setExpandedId(null), []);
    const handleSaveStart = useCallback((id) => setPendingSaveId(id), []);
    const handleSaveEnd   = useCallback(() => setPendingSaveId(null), []);

    const handleSave = async (itemId, draft) => {
        // 1) Поля питання
        const r = await api.put(`/api/projects/${pid}/categories/${cid}/items/${itemId}`, toPayload(draft));
        let updated = r.data;

        // 2) Застосовуємо чорнові зміни фото (тільки тут, при збереженні)
        if (draft._imageDelete) {
            await api.delete(`/api/projects/${pid}/categories/${cid}/items/${itemId}/image`);
            updated = { ...updated, answer: { ...updated.answer, image_path: '' } };
        } else if (draft._imageFile) {
            const fd = new FormData();
            fd.append('image', draft._imageFile);
            await api.post(`/api/projects/${pid}/categories/${cid}/items/${itemId}/image`, fd,
                { headers: { 'Content-Type': 'multipart/form-data' } });
            // Шлях детермінований; версію збиваємо нижче, щоб фото оновилось одразу
            updated = { ...updated, answer: { ...updated.answer, image_path: `/answers/${pid}/${cid}/${itemId}.jpg` } };
        }

        setItems(prev => prev.map(i => i.id === itemId ? updated : i));
        if (draft._imageFile || draft._imageDelete) {
            setImageVersions(prev => ({ ...prev, [itemId]: Date.now() }));
        }
        setLiveEdits(prev => { const n = { ...prev }; delete n[itemId]; return n; });
        return updated;
    };

    const handleDelete = async (itemId) => {
        if (!confirm('Видалити питання?')) return;
        await api.delete(`/api/projects/${pid}/categories/${cid}/items/${itemId}`);
        setItems(prev => prev.filter(i => i.id !== itemId));
        setLiveEdits(prev => { const n = { ...prev }; delete n[itemId]; return n; });
    };

    const handleRender = async (itemId) => {
        try {
            await api.post(`/api/projects/${pid}/categories/${cid}/items/${itemId}/render`);
            setItems(prev => prev.map(i => i.id === itemId
                ? { ...i, video: { ...i.video, render_status: 'rendering' } } : i));
        } catch (err) { alert(err.response?.data?.error || 'Помилка рендеру'); }
    };

    const handleMove = async (item, direction) => {
        const sorted = [...items].sort((a, b) => a.position - b.position);
        const idx = sorted.findIndex(i => i.id === item.id);
        const newPos = direction === 'up' ? idx - 1 : idx + 1;
        if (newPos < 0 || newPos >= sorted.length) return;
        try {
            await api.put(`/api/projects/${pid}/categories/${cid}/items/${item.id}`, { position: newPos });
            const r = await api.get(`/api/projects/${pid}/categories/${cid}/items`);
            setItems(r.data || []);
        } catch { alert('Не вдалося змінити порядок'); }
    };

    const handleQuickToggleShowVideo = async (itemId, newVal) => {
        try {
            const r = await api.put(`/api/projects/${pid}/categories/${cid}/items/${itemId}`, { show_video: newVal });
            setItems(prev => prev.map(i => i.id === itemId ? r.data : i));
        } catch { alert('Не вдалося змінити статус'); }
    };

    // ── Render ────────────────────────────────────────────────────────────────

    if (loading) return <p>Завантаження...</p>;
    if (error)   return <div><p style={{ color: 'red' }}>{error}</p><Link to={`/projects/${pid}`}>← Назад</Link></div>;

    const sortedItems = [...items].sort((a, b) => a.position - b.position);
    const wsLabel = { connected: '🟢 онлайн', disconnected: '🔴 офлайн', connecting: '🟡 підключення...' }[wsStatus] ?? wsStatus;

    return (
        <div style={{ padding: '16px', maxWidth: '1200px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <p style={{ margin: 0 }}>
                    <a href="/" onClick={e => { e.preventDefault(); guardedNav('/'); }}>Проєкти</a>{' / '}
                    <a href={`/projects/${pid}`} onClick={e => { e.preventDefault(); guardedNav(`/projects/${pid}`); }}>Проєкт #{pid}</a>{' / '}
                    <strong>{catTitle}</strong>
                    {pageDirty && <span style={{ color: '#c80', marginLeft: 8, fontSize: '0.85em' }}>● незбережені зміни</span>}
                </p>
                <small style={{ color: '#888' }} title="WebSocket live-sync">
                    {wsLabel}
                    {wsStatus === 'disconnected' && ' — live-зміни не видно'}
                </small>
            </div>

            <h1 style={{ marginBottom: '4px' }}>{catTitle}</h1>
            <p style={{ margin: '0 0 16px 0', color: '#888' }}>
                {items.length} питань
                {items.some(i => i.video?.media?.status === 'downloading') && ' · ⏳ завантаження відео...'}
                {items.some(i => i.video?.render_status === 'rendering') && ' · ⏳ рендер...'}
                {Object.keys(liveEdits).length > 0 && ` · ✏️ ${Object.keys(liveEdits).length} редагується live`}
            </p>

            <hr />

            {!isCreating ? (
                <button onClick={() => setIsCreating(true)}>+ Нове питання</button>
            ) : (
                <CreateForm pid={pid} cid={cid}
                    onCreate={item => { setItems(p => [...p, item]); setIsCreating(false); }}
                    onCancel={() => setIsCreating(false)}
                    onDirty={setCreateDirty} />
            )}

            <div style={{ marginTop: '16px' }}>
                {sortedItems.length === 0 ? (
                    <p style={{ color: '#888' }}>Питань ще немає.</p>
                ) : (
                    sortedItems.map((item, idx) => (
                        <ItemCard
                            key={item.id}
                            item={item} idx={idx} total={sortedItems.length}
                            pid={pid} cid={cid}
                            isExpanded={expandedId === item.id}
                            liveEdit={liveEdits[item.id] ?? null}
                            renderVersion={renderVersions[item.id] ?? 0}
                            imageVersion={imageVersions[item.id] ?? 0}
                            onOpen={handleOpen} onClose={handleClose}
                            onSaveStart={handleSaveStart} onSaveEnd={handleSaveEnd}
                            onDraftChange={handleDraftChange} onDirtyChange={handleDirtyChange}
                            onSave={handleSave} onDelete={handleDelete}
                            onRender={handleRender} onMove={handleMove}
                            onQuickToggleShowVideo={handleQuickToggleShowVideo}
                        />
                    ))
                )}
            </div>
        </div>
    );
}
