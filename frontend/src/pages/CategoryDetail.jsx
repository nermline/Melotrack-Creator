import { useEffect, useState, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api';

// ─── Helpers ─────────────────────────────────────────────────────────────────

const API_BASE = 'http://localhost:8080';

function extractYouTubeId(url) {
    if (!url) return null;
    const m = url.match(/(?:v=|youtu\.be\/|embed\/)([a-zA-Z0-9_-]{11})/);
    return m ? m[1] : null;
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
            youtube_url: f.youtube_url,
            start_time: parseFloat(f.start_time) || 0,
            end_time:   parseFloat(f.end_time)   || 0,
            volume:     parseFloat(f.volume)     || 1,
            crop_x:     parseInt(f.crop_x)       || 0,
            crop_y:     parseInt(f.crop_y)        || 0,
            crop_width: parseInt(f.crop_width)   || 0,
            crop_height:parseInt(f.crop_height)  || 0,
        },
        answer: { title: f.answer_title },
    };
}

function renderBadge(status) {
    const map = { ready: '✅ готовий', rendering: '⏳ рендер...', unrendered: '⬜ не рендеровано', error: '❌ помилка' };
    return map[status] ?? status ?? '—';
}

function mediaBadge(status) {
    const map = { ready: '✅ завантажено', downloading: '⏳ завантаж...', error: '❌ помилка' };
    return map[status] ?? status ?? '—';
}

// ─── Create form (simple, always open above the list) ────────────────────────

function CreateForm({ pid, cid, onCreate, onCancel }) {
    const [form, setForm] = useState(EMPTY_FORM);
    const [error, setError] = useState('');
    const set = (k, v) => setForm(p => ({ ...p, [k]: v }));

    const handleSubmit = async (e) => {
        e.preventDefault();
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
        <form onSubmit={handleSubmit} style={{ border: '2px solid #555', padding: '12px', margin: '8px 0', background: '#f9f9f9' }}>
            <strong>Нове питання</strong>
            <br /><br />
            <table>
                <tbody>
                    <tr>
                        <td>YouTube URL:</td>
                        <td><input style={{ width: '360px' }} value={form.youtube_url}
                            onChange={e => set('youtube_url', e.target.value)} placeholder="https://youtube.com/watch?v=..." required /></td>
                    </tr>
                    <tr>
                        <td>Відповідь:</td>
                        <td><input style={{ width: '360px' }} value={form.answer_title}
                            onChange={e => set('answer_title', e.target.value)} placeholder="Текст відповіді" /></td>
                    </tr>
                    <tr>
                        <td>Показ відео:</td>
                        <td><input type="checkbox" checked={form.show_video} onChange={e => set('show_video', e.target.checked)} /></td>
                    </tr>
                    <tr>
                        <td>Початок / Кінець (сек):</td>
                        <td>
                            <input type="number" step="0.1" value={form.start_time} onChange={e => set('start_time', e.target.value)} style={{ width: '80px' }} />
                            {' — '}
                            <input type="number" step="0.1" value={form.end_time} onChange={e => set('end_time', e.target.value)} style={{ width: '80px' }} />
                        </td>
                    </tr>
                    <tr>
                        <td>Гучність:</td>
                        <td>
                            <input type="range" min="0" max="2" step="0.01"
                                value={form.volume} onChange={e => set('volume', parseFloat(e.target.value))}
                                style={{ width: '160px' }} />
                            {' '}<strong>{Number(form.volume).toFixed(2)}</strong>
                        </td>
                    </tr>
                    <tr>
                        <td>Crop X / Y:</td>
                        <td>
                            <input type="number" value={form.crop_x} onChange={e => set('crop_x', e.target.value)} style={{ width: '70px' }} />
                            {' / '}
                            <input type="number" value={form.crop_y} onChange={e => set('crop_y', e.target.value)} style={{ width: '70px' }} />
                        </td>
                    </tr>
                    <tr>
                        <td>Crop W / H:</td>
                        <td>
                            <input type="number" value={form.crop_width} onChange={e => set('crop_width', e.target.value)} style={{ width: '70px' }} />
                            {' / '}
                            <input type="number" value={form.crop_height} onChange={e => set('crop_height', e.target.value)} style={{ width: '70px' }} />
                        </td>
                    </tr>
                </tbody>
            </table>
            {error && <p style={{ color: 'red' }}>{error}</p>}
            <button type="submit">Створити</button>
            {' '}
            <button type="button" onClick={onCancel}>Скасувати</button>
        </form>
    );
}

// ─── Single item card ─────────────────────────────────────────────────────────

function ItemCard({ item, idx, total, pid, cid, onSave, onDelete, onRender, onMove, onUploadImage, onDeleteImage, onQuickToggleShowVideo }) {
    const [expanded, setExpanded] = useState(false);
    const [draft, setDraft]       = useState(null);
    const [saving, setSaving]     = useState(false);
    const [error, setError]       = useState('');

    const set = (k, v) => setDraft(p => ({ ...p, [k]: v }));

    const token = localStorage.getItem('token');

    // Rendered video URL (uses query param token — supported by backend TokenLookup)
    const renderedVideoUrl = item.video?.render_status === 'ready'
        ? `${API_BASE}/media/${pid}/${cid}/${item.id}.mp4?token=${token}`
        : null;

    // YouTube embed (at start_time)
    const ytId = extractYouTubeId(item.video?.youtube_url);
    // When draft exists, reflect draft's start_time in embed so user sees what they set
    const embedStartTime = expanded && draft ? Math.floor(parseFloat(draft.start_time) || 0) : Math.floor(item.video?.start_time || 0);
    const embedUrl = ytId ? `https://www.youtube.com/embed/${ytId}?start=${embedStartTime}&rel=0` : null;

    const toggleExpand = () => {
        if (!expanded) setDraft(itemToForm(item));
        setExpanded(e => !e);
        setError('');
    };

    const handleSave = async () => {
        setSaving(true);
        setError('');
        try {
            const updated = await onSave(item.id, draft);
            // Reflect saved data in draft so next expand is fresh
            setDraft(itemToForm(updated));
            setExpanded(false);
        } catch (e) {
            setError(e.message || 'Помилка збереження');
        } finally {
            setSaving(false);
        }
    };

    const handleUpload = () => {
        const inp = document.createElement('input');
        inp.type = 'file';
        inp.accept = 'image/*';
        inp.onchange = e => { if (inp.files[0]) onUploadImage(item.id, inp.files[0]); };
        inp.click();
    };

    return (
        <div style={{ border: '1px solid #bbb', margin: '10px 0' }}>
            {/* ── Collapsed summary row ─────────────────── */}
            <div style={{ padding: '8px 12px', display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', background: expanded ? '#f0f0f0' : 'transparent' }}>
                {/* Position */}
                <span style={{ minWidth: '30px', color: '#888' }}>#{item.position + 1}</span>

                {/* Move */}
                <span>
                    <button onClick={() => onMove(item, 'up')} disabled={idx === 0} title="Вгору">↑</button>
                    <button onClick={() => onMove(item, 'down')} disabled={idx === total - 1} title="Вниз">↓</button>
                </span>

                {/* Answer title */}
                <strong style={{ minWidth: '120px' }}>{item.answer?.title || <em style={{ color: '#aaa' }}>без відповіді</em>}</strong>

                {/* Quick show_video toggle */}
                <label title="Показ відео у грі" style={{ cursor: 'pointer', userSelect: 'none' }}>
                    <input type="checkbox"
                        checked={item.show_video}
                        onChange={e => onQuickToggleShowVideo(item.id, e.target.checked)}
                        style={{ marginRight: '4px' }}
                    />
                    відео
                </label>

                {/* Статуси */}
                <span style={{ fontSize: '0.85em', color: '#555' }}>{mediaBadge(item.video?.media?.status)}</span>
                <span style={{ fontSize: '0.85em', color: '#555' }}>{renderBadge(item.video?.render_status)}</span>

                {/* Actions */}
                <button onClick={toggleExpand}>{expanded ? '▲ Згорнути' : '▼ Розгорнути'}</button>

                {item.video?.media?.status === 'ready' && item.video?.render_status !== 'rendering' && (
                    <button onClick={() => onRender(item.id)}>▶ Рендер</button>
                )}

                <button onClick={handleUpload} title="Завантажити фото відповіді">🖼</button>

                {item.answer?.image_path && (
                    <button onClick={() => onDeleteImage(item.id)} style={{ color: 'red' }} title="Видалити фото">🗑🖼</button>
                )}

                <button onClick={() => onDelete(item.id)} style={{ color: 'red' }}>🗑 Видалити</button>
            </div>

            {/* ── Expanded detail panel ──────────────────── */}
            {expanded && draft && (
                <div style={{ padding: '12px', borderTop: '1px solid #bbb' }}>

                    {/* Video previews */}
                    <div style={{ display: 'flex', gap: '24px', marginBottom: '16px', flexWrap: 'wrap' }}>
                        {/* Original YouTube */}
                        <div>
                            <p style={{ margin: '0 0 4px 0' }}>
                                <strong>Оригінал</strong>
                                {' '}
                                <small style={{ color: '#888' }}>
                                    (з {Number(draft.start_time).toFixed(1)}s — оновиться після збереження)
                                </small>
                            </p>
                            {embedUrl ? (
                                <iframe
                                    key={`yt-${item.id}-${embedStartTime}`}
                                    width="480" height="270"
                                    src={embedUrl}
                                    title="YouTube preview"
                                    frameBorder="0"
                                    allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
                                    allowFullScreen
                                />
                            ) : (
                                <p style={{ color: '#aaa', width: '480px', height: '270px', border: '1px solid #ddd', display: 'flex', alignItems: 'center', justifyContent: 'center', margin: 0 }}>
                                    Вкажіть YouTube URL
                                </p>
                            )}
                        </div>

                        {/* Rendered clip */}
                        <div>
                            <p style={{ margin: '0 0 4px 0' }}>
                                <strong>Відрендерований кліп</strong>
                            </p>
                            {renderedVideoUrl ? (
                                <video
                                    key={renderedVideoUrl}
                                    width="480" height="270"
                                    controls
                                    style={{ background: '#000' }}
                                    src={renderedVideoUrl}
                                />
                            ) : (
                                <div style={{ width: '480px', height: '270px', border: '1px solid #ddd', background: '#fafafa', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                                    <span style={{ color: '#aaa' }}>
                                        {item.video?.render_status === 'rendering' ? '⏳ рендериться...' : 'ще не відрендеровано'}
                                    </span>
                                </div>
                            )}
                        </div>
                    </div>

                    {/* Edit fields */}
                    <table>
                        <tbody>
                            <tr>
                                <td style={{ paddingRight: '12px', paddingBottom: '8px' }}>YouTube URL:</td>
                                <td style={{ paddingBottom: '8px' }}>
                                    <input style={{ width: '420px' }} value={draft.youtube_url}
                                        onChange={e => set('youtube_url', e.target.value)} />
                                </td>
                            </tr>
                            <tr>
                                <td style={{ paddingRight: '12px', paddingBottom: '8px' }}>Відповідь:</td>
                                <td style={{ paddingBottom: '8px' }}>
                                    <input style={{ width: '420px' }} value={draft.answer_title}
                                        onChange={e => set('answer_title', e.target.value)} />
                                </td>
                            </tr>
                            <tr>
                                <td style={{ paddingRight: '12px', paddingBottom: '8px' }}>Показ відео:</td>
                                <td style={{ paddingBottom: '8px' }}>
                                    <input type="checkbox" checked={draft.show_video}
                                        onChange={e => set('show_video', e.target.checked)} />
                                    <label style={{ marginLeft: '6px' }}>
                                        {draft.show_video ? 'відео показується гравцям' : 'відео приховано від гравців'}
                                    </label>
                                </td>
                            </tr>
                            <tr>
                                <td style={{ paddingRight: '12px', paddingBottom: '8px' }}>
                                    Часові мітки (сек):
                                </td>
                                <td style={{ paddingBottom: '8px' }}>
                                    <label>Початок:</label>
                                    {' '}
                                    <input type="number" step="0.1" min="0" style={{ width: '90px' }}
                                        value={draft.start_time}
                                        onChange={e => set('start_time', e.target.value)} />
                                    {' — '}
                                    <label>Кінець:</label>
                                    {' '}
                                    <input type="number" step="0.1" min="0" style={{ width: '90px' }}
                                        value={draft.end_time}
                                        onChange={e => set('end_time', e.target.value)} />
                                    {' '}
                                    <small style={{ color: '#888' }}>
                                        тривалість: {Math.max(0, (parseFloat(draft.end_time) || 0) - (parseFloat(draft.start_time) || 0)).toFixed(1)}s
                                    </small>
                                </td>
                            </tr>
                            <tr>
                                <td style={{ paddingRight: '12px', paddingBottom: '8px' }}>Гучність:</td>
                                <td style={{ paddingBottom: '8px' }}>
                                    <input type="range" min="0" max="2" step="0.01" style={{ width: '220px', verticalAlign: 'middle' }}
                                        value={draft.volume}
                                        onChange={e => set('volume', parseFloat(e.target.value))} />
                                    {' '}
                                    <strong>{Number(draft.volume).toFixed(2)}</strong>
                                    {' '}
                                    <input type="number" min="0" max="2" step="0.01" style={{ width: '65px' }}
                                        value={draft.volume}
                                        onChange={e => set('volume', parseFloat(e.target.value) || 0)} />
                                    {' '}
                                    <small style={{ color: '#888' }}>
                                        {draft.volume < 0.5 ? '(тихо)' : draft.volume > 1.5 ? '(гучно)' : ''}
                                    </small>
                                </td>
                            </tr>
                            <tr>
                                <td style={{ paddingRight: '12px', paddingBottom: '8px' }}>Обрізання (crop):</td>
                                <td style={{ paddingBottom: '8px' }}>
                                    <table style={{ borderCollapse: 'collapse' }}>
                                        <tbody>
                                            <tr>
                                                <td style={{ paddingRight: '8px' }}>X:</td>
                                                <td><input type="number" value={draft.crop_x} style={{ width: '75px' }}
                                                    onChange={e => set('crop_x', e.target.value)} /></td>
                                                <td style={{ paddingLeft: '12px', paddingRight: '8px' }}>Y:</td>
                                                <td><input type="number" value={draft.crop_y} style={{ width: '75px' }}
                                                    onChange={e => set('crop_y', e.target.value)} /></td>
                                            </tr>
                                            <tr>
                                                <td style={{ paddingRight: '8px' }}>Ширина:</td>
                                                <td><input type="number" value={draft.crop_width} style={{ width: '75px' }}
                                                    onChange={e => set('crop_width', e.target.value)} /></td>
                                                <td style={{ paddingLeft: '12px', paddingRight: '8px' }}>Висота:</td>
                                                <td><input type="number" value={draft.crop_height} style={{ width: '75px' }}
                                                    onChange={e => set('crop_height', e.target.value)} /></td>
                                            </tr>
                                        </tbody>
                                    </table>
                                    {(parseInt(draft.crop_width) > 0 && parseInt(draft.crop_height) > 0) && (
                                        <small style={{ color: '#888' }}>
                                            crop={draft.crop_width}:{draft.crop_height}:{draft.crop_x}:{draft.crop_y}
                                        </small>
                                    )}
                                    {(parseInt(draft.crop_width) === 0 || parseInt(draft.crop_height) === 0) && (
                                        <small style={{ color: '#aaa' }}>без обрізання (W або H = 0)</small>
                                    )}
                                </td>
                            </tr>
                        </tbody>
                    </table>

                    {error && <p style={{ color: 'red' }}>{error}</p>}

                    <div style={{ marginTop: '10px' }}>
                        <button onClick={handleSave} disabled={saving}>
                            {saving ? 'Збереження...' : 'Зберегти зміни'}
                        </button>
                        {' '}
                        <button onClick={() => { setExpanded(false); setError(''); }}>Скасувати</button>
                    </div>
                </div>
            )}
        </div>
    );
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function CategoryDetail() {
    const { pid, cid } = useParams();

    const [items, setItems]       = useState([]);
    const [catTitle, setCatTitle] = useState('');
    const [loading, setLoading]   = useState(true);
    const [error, setError]       = useState('');
    const [isCreating, setIsCreating] = useState(false);

    const pollingRef = useRef(null);

    useEffect(() => {
        loadItems();
        return () => clearInterval(pollingRef.current);
    }, [pid, cid]);

    // Auto-poll while any item is still downloading or rendering
    useEffect(() => {
        const hasPending = items.some(i =>
            i.video?.media?.status === 'downloading' ||
            i.video?.render_status === 'rendering'
        );
        if (hasPending && !pollingRef.current) {
            pollingRef.current = setInterval(loadItems, 4000);
        } else if (!hasPending && pollingRef.current) {
            clearInterval(pollingRef.current);
            pollingRef.current = null;
        }
    }, [items]);

    const loadItems = async () => {
        try {
            if (!catTitle) {
                const projRes = await api.get(`/api/projects/${pid}`);
                const cat = (projRes.data.categories || []).find(c => String(c.id) === String(cid));
                setCatTitle(cat?.title ?? `Категорія #${cid}`);
            }
            const r = await api.get(`/api/projects/${pid}/categories/${cid}/items`);
            setItems(r.data || []);
        } catch (err) {
            if (err.response?.status === 404) setError('Категорію не знайдено');
            else setError('Помилка завантаження');
        } finally {
            setLoading(false);
        }
    };

    // ── Handlers passed to ItemCard ──────────────────────────────────────────

    const handleSave = async (itemId, draft) => {
        const r = await api.put(`/api/projects/${pid}/categories/${cid}/items/${itemId}`, toPayload(draft));
        setItems(prev => prev.map(i => i.id === itemId ? r.data : i));
        return r.data;
    };

    const handleDelete = async (itemId) => {
        if (!confirm('Видалити питання?')) return;
        await api.delete(`/api/projects/${pid}/categories/${cid}/items/${itemId}`);
        setItems(prev => prev.filter(i => i.id !== itemId));
    };

    const handleRender = async (itemId) => {
        try {
            await api.post(`/api/projects/${pid}/categories/${cid}/items/${itemId}/render`);
            setItems(prev => prev.map(i => i.id === itemId
                ? { ...i, video: { ...i.video, render_status: 'rendering' } }
                : i
            ));
        } catch (err) {
            alert(err.response?.data?.error || 'Помилка рендеру');
        }
    };

    const handleMove = async (item, direction) => {
        const sorted = [...items].sort((a, b) => a.position - b.position);
        const idx = sorted.findIndex(i => i.id === item.id);
        const newPos = direction === 'up' ? idx - 1 : idx + 1;
        if (newPos < 0 || newPos >= sorted.length) return;
        try {
            await api.put(`/api/projects/${pid}/categories/${cid}/items/${item.id}`, { position: newPos });
            await loadItems();
        } catch {
            alert('Не вдалося змінити порядок');
        }
    };

    const handleUploadImage = async (itemId, file) => {
        const fd = new FormData();
        fd.append('image', file);
        try {
            await api.post(`/api/projects/${pid}/categories/${cid}/items/${itemId}/image`, fd,
                { headers: { 'Content-Type': 'multipart/form-data' } });
            await loadItems();
        } catch (err) {
            alert(err.response?.data?.error || 'Помилка завантаження фото');
        }
    };

    const handleDeleteImage = async (itemId) => {
        if (!confirm('Видалити фото відповіді?')) return;
        try {
            await api.delete(`/api/projects/${pid}/categories/${cid}/items/${itemId}/image`);
            await loadItems();
        } catch {
            alert('Не вдалося видалити фото');
        }
    };

    // Quick toggle show_video without opening the full form
    const handleQuickToggleShowVideo = async (itemId, newVal) => {
        try {
            const r = await api.put(`/api/projects/${pid}/categories/${cid}/items/${itemId}`, { show_video: newVal });
            setItems(prev => prev.map(i => i.id === itemId ? r.data : i));
        } catch {
            alert('Не вдалося змінити статус');
        }
    };

    // ── Render ───────────────────────────────────────────────────────────────

    if (loading) return <p>Завантаження...</p>;
    if (error)   return <div><p style={{ color: 'red' }}>{error}</p><Link to={`/projects/${pid}`}>← Назад</Link></div>;

    const sortedItems = [...items].sort((a, b) => a.position - b.position);

    return (
        <div style={{ padding: '16px', maxWidth: '1100px' }}>
            <p>
                <Link to="/">Проєкти</Link>{' / '}
                <Link to={`/projects/${pid}`}>Проєкт #{pid}</Link>{' / '}
                <strong>{catTitle}</strong>
            </p>

            <h1 style={{ marginBottom: '4px' }}>{catTitle}</h1>
            <p style={{ margin: '0 0 16px 0', color: '#888' }}>
                {items.length} питань
                {items.some(i => i.video?.media?.status === 'downloading') && ' · ⏳ є завантаження...'}
                {items.some(i => i.video?.render_status === 'rendering') && ' · ⏳ є рендер...'}
            </p>

            <hr />

            {!isCreating ? (
                <button onClick={() => setIsCreating(true)}>+ Нове питання</button>
            ) : (
                <CreateForm pid={pid} cid={cid} onCreate={item => { setItems(p => [...p, item]); setIsCreating(false); }} onCancel={() => setIsCreating(false)} />
            )}

            <div style={{ marginTop: '16px' }}>
                {sortedItems.length === 0 ? (
                    <p style={{ color: '#888' }}>Питань ще немає.</p>
                ) : (
                    sortedItems.map((item, idx) => (
                        <ItemCard
                            key={item.id}
                            item={item}
                            idx={idx}
                            total={sortedItems.length}
                            pid={pid}
                            cid={cid}
                            onSave={handleSave}
                            onDelete={handleDelete}
                            onRender={handleRender}
                            onMove={handleMove}
                            onUploadImage={handleUploadImage}
                            onDeleteImage={handleDeleteImage}
                            onQuickToggleShowVideo={handleQuickToggleShowVideo}
                        />
                    ))
                )}
            </div>
        </div>
    );
}
