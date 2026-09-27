import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { AnimatePresence, motion, Reorder, useDragControls } from 'framer-motion';
import {
    CircleAlert,
    Eye,
    EyeOff,
    FileAudio,
    GripVertical,
    ListMusic,
    Pause,
    Pencil,
    Play,
    Plus,
    RefreshCw,
    Trash2,
} from 'lucide-react';
import { api, upload } from '../lib/api';
import { fmtTime, songs } from '../lib/format';
import { Button, Chip, Empty, InlineEdit, ProgressBar, Spinner } from '../ui';
import { cx } from '../lib/cx';
import { useFeedback } from '../ui/feedbackContext';
import { songState } from './useProjectStore';
import { stopPreview, togglePreview, usePreview } from './preview';
import SongEditor from './SongEditor';

export default function ProgramTab({ store }) {
    const { project } = store;
    const [params, setParams] = useSearchParams();
    const { toast, confirm } = useFeedback();
    const [cats, setCats] = useState(project.categories);
    const [editing, setEditing] = useState(null);
    const [newCat, setNewCat] = useState('');
    const dragging = useRef(false);

    useEffect(() => {
        if (!dragging.current) setCats(project.categories);
    }, [project.categories]);

    const selectedId = Number(params.get('c')) || project.categories[0]?.id;
    const selected = project.categories.find((c) => c.id === selectedId) || project.categories[0];
    const select = (id) => setParams({ c: String(id) }, { replace: true });

    const addCategory = async (e) => {
        e.preventDefault();
        const title = newCat.trim();
        if (!title) return;
        try {
            const c = await api.post(`/api/projects/${project.id}/categories`, { title });
            setNewCat('');
            await store.reload();
            select(c.id);
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const commitCatOrder = async () => {
        dragging.current = false;
        const ids = cats.map((c) => c.id);
        if (ids.join() === project.categories.map((c) => c.id).join()) return;
        try {
            await api.put(`/api/projects/${project.id}/categories/order`, { ids });
        } catch (err) {
            toast(err.message, 'bad');
        }
        store.reload();
    };

    const renameCategory = async (cat, title) => {
        if (!title) return;
        try {
            await api.patch(`/api/categories/${cat.id}`, { title });
            store.reload();
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const deleteCategory = async (cat) => {
        const ok = await confirm({
            title: 'Видалити категорію?',
            message: `«${cat.title}» і ${songs(cat.items.length)} в ній буде видалено разом з балами за них.`,
            confirmLabel: 'Видалити',
            danger: true,
        });
        if (!ok) return;
        try {
            await api.del(`/api/categories/${cat.id}`);
            store.reload();
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const editingItem = editing && project.categories.flatMap((c) => c.items).find((i) => i.id === editing);

    return (
        <div className="grid gap-5 lg:grid-cols-[300px_minmax(0,1fr)]">
            <aside className="lg:sticky lg:top-[120px] lg:self-start">
                <div className="mb-2 flex items-center justify-between">
                    <div className="eyebrow">Категорії</div>
                    <span className="text-xs text-faint">{cats.length}</span>
                </div>
                <Reorder.Group
                    axis="y"
                    values={cats}
                    onReorder={(v) => {
                        dragging.current = true;
                        setCats(v);
                    }}
                    className="m-0 flex list-none flex-col gap-1.5 p-0"
                >
                    {cats.map((c, i) => (
                        <CategoryRow
                            key={c.id}
                            cat={c}
                            index={i}
                            media={project.media}
                            active={selected?.id === c.id}
                            onSelect={() => select(c.id)}
                            onDragEnd={commitCatOrder}
                        />
                    ))}
                </Reorder.Group>
                <form onSubmit={addCategory} className="mt-3 flex gap-2">
                    <input
                        className="field"
                        placeholder="Нова категорія…"
                        maxLength={120}
                        value={newCat}
                        onChange={(e) => setNewCat(e.target.value)}
                    />
                    <Button type="submit" icon={Plus} disabled={!newCat.trim()} aria-label="Додати категорію" />
                </form>
            </aside>

            <section className="min-w-0">
                {!selected ? (
                    <div className="panel">
                        <Empty icon={ListMusic} title="Додайте першу категорію">
                            Категорія — це тема раунду: «Хіти 2000-х», «Саундтреки», «Українська естрада»… У кожній —
                            кілька пісень.
                        </Empty>
                    </div>
                ) : (
                    <CategoryView
                        key={selected.id}
                        cat={selected}
                        store={store}
                        onRename={(t) => renameCategory(selected, t)}
                        onDelete={() => deleteCategory(selected)}
                        onEdit={(id) => {
                            stopPreview();
                            setEditing(id);
                        }}
                    />
                )}
            </section>

            <SongEditor
                open={!!editingItem}
                item={editingItem}
                media={editingItem && project.media[editingItem.media_id]}
                store={store}
                onClose={() => setEditing(null)}
            />
        </div>
    );
}

function readiness(cat, media) {
    let bad = 0;
    let busy = 0;
    for (const it of cat.items) {
        const s = songState(it, media[it.media_id]).key;
        if (s === 'download_error' || s === 'clip_error') bad++;
        else if (s !== 'ready') busy++;
    }
    return { bad, busy };
}

function CategoryRow({ cat, index, media, active, onSelect, onDragEnd }) {
    const controls = useDragControls();
    const { bad, busy } = readiness(cat, media);
    return (
        <Reorder.Item
            value={cat}
            dragListener={false}
            dragControls={controls}
            onDragEnd={onDragEnd}
            className={cx(
                'flex cursor-pointer items-center gap-2 rounded-xl border px-2 py-2.5 transition-colors',
                active ? 'border-accent/50 bg-accent/10' : 'border-line bg-panel hover:border-line-strong',
            )}
            onClick={onSelect}
        >
            <span
                className="cursor-grab touch-none px-0.5 text-faint active:cursor-grabbing"
                onPointerDown={(e) => controls.start(e)}
                onClick={(e) => e.stopPropagation()}
                title="Перетягніть, щоб змінити порядок"
            >
                <GripVertical size={16} />
            </span>
            <span className="w-4 text-center text-xs font-bold text-faint">{index + 1}</span>
            <span className="min-w-0 flex-1 truncate text-sm font-semibold">{cat.title}</span>
            {bad > 0 && <CircleAlert size={15} className="text-bad" title="Є пісні з помилками" />}
            {bad === 0 && busy > 0 && <Spinner size={13} className="text-info" />}
            <span className="text-xs text-faint">{cat.items.length}</span>
        </Reorder.Item>
    );
}

function CategoryView({ cat, store, onRename, onDelete, onEdit }) {
    const { project } = store;
    const { toast } = useFeedback();
    const [items, setItems] = useState(cat.items);
    const dragging = useRef(false);

    useEffect(() => {
        if (!dragging.current) setItems(cat.items);
    }, [cat.items]);

    const commitOrder = async () => {
        dragging.current = false;
        const ids = items.map((i) => i.id);
        if (ids.join() === cat.items.map((i) => i.id).join()) return;
        try {
            await api.put(`/api/categories/${cat.id}/items/order`, { ids });
        } catch (err) {
            toast(err.message, 'bad');
        }
        store.reload();
    };

    return (
        <div>
            <div className="mb-4 flex flex-wrap items-center gap-3">
                <div className="min-w-0 flex-1">
                    <div className="eyebrow">Категорія</div>
                    <InlineEdit
                        value={cat.title}
                        onSave={onRename}
                        maxLength={120}
                        className="mt-0.5 block max-w-full font-display text-xl font-bold sm:text-2xl"
                        inputClassName="mt-1 max-w-md text-lg"
                    />
                    <div className="mt-1 text-sm text-dim">
                        {songs(cat.items.length)}
                    </div>
                </div>
                <Button variant="danger" size="sm" icon={Trash2} onClick={onDelete}>
                    Видалити категорію
                </Button>
            </div>

            <AddSongs cat={cat} store={store} />

            {items.length === 0 ? (
                <div className="panel mt-4">
                    <Empty icon={ListMusic} title="Тут поки тихо">
                        Вставте посилання на YouTube вище — назва пісні й обкладинка підтягнуться автоматично.
                    </Empty>
                </div>
            ) : (
                <Reorder.Group
                    axis="y"
                    values={items}
                    onReorder={(v) => {
                        dragging.current = true;
                        setItems(v);
                    }}
                    className="m-0 mt-4 flex list-none flex-col gap-2 p-0"
                >
                    {items.map((it, i) => (
                        <SongRow
                            key={it.id}
                            item={it}
                            index={i}
                            media={project.media[it.media_id]}
                            mediaProgress={store.mediaProgress[it.media_id]}
                            clipProgress={store.clipProgress[it.id]}
                            store={store}
                            onEdit={() => onEdit(it.id)}
                            onDragEnd={commitOrder}
                        />
                    ))}
                </Reorder.Group>
            )}
        </div>
    );
}

function AddSongs({ cat, store }) {
    const [text, setText] = useState('');
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(null);
    const fileRef = useRef(null);
    const { toast } = useFeedback();
    const lines = text.split('\n').filter((l) => l.trim()).length;

    const add = async (e) => {
        e?.preventDefault();
        if (!text.trim()) return;
        setBusy(true);
        try {
            const res = await api.post(`/api/categories/${cat.id}/items`, { urls: [text] });
            setText('');
            await store.reload();
            const n = res.items.length;
            toast(
                `Додано ${songs(n)}. Завантаження йде у фоні.` +
                    (res.invalid.length ? `\nПропущено рядків без посилання: ${res.invalid.length}` : ''),
                res.invalid.length ? 'info' : 'ok',
            );
        } catch (err) {
            toast(err.message, 'bad');
        } finally {
            setBusy(false);
        }
    };

    const onFile = async (e) => {
        const file = e.target.files?.[0];
        e.target.value = '';
        if (!file) return;
        const fd = new FormData();
        fd.append('file', file);
        setUploading(0);
        try {
            await upload(`/api/categories/${cat.id}/items/upload`, fd, setUploading);
            await store.reload();
            toast('Файл завантажено, обробка йде у фоні');
        } catch (err) {
            toast(err.message, 'bad');
        } finally {
            setUploading(null);
        }
    };

    return (
        <form onSubmit={add} className="panel p-3">
            <textarea
                className="field min-h-[46px] resize-y font-mono text-[13px]"
                rows={lines > 1 ? Math.min(lines + 1, 8) : 1}
                placeholder="Вставте посилання на YouTube — можна одразу кілька, кожне з нового рядка"
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) add();
                }}
                onPaste={(e) => {
                    // Pasting a single link into an empty box adds it right away.
                    const pasted = e.clipboardData.getData('text');
                    if (!text && !pasted.includes('\n') && /youtu/.test(pasted)) {
                        e.preventDefault();
                        setText(pasted);
                        setTimeout(() => document.getElementById(`add-${cat.id}`)?.click(), 0);
                    }
                }}
            />
            <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
                <span className="text-xs text-faint">
                    {lines > 1 ? `${lines} рядків` : 'Ctrl/⌘ + Enter — додати'}
                </span>
                <div className="flex gap-2">
                    <input ref={fileRef} type="file" accept="video/*,audio/*" hidden onChange={onFile} />
                    <Button
                        size="sm"
                        variant="ghost"
                        icon={FileAudio}
                        loading={uploading !== null}
                        onClick={() => fileRef.current?.click()}
                        title="Пісні немає на YouTube? Завантажте аудіо чи відео з компʼютера"
                    >
                        {uploading !== null ? `${Math.round(uploading * 100)}%` : 'З файлу'}
                    </Button>
                    <Button id={`add-${cat.id}`} type="submit" size="sm" variant="primary" icon={Plus} loading={busy} disabled={!text.trim()}>
                        Додати
                    </Button>
                </div>
            </div>
        </form>
    );
}

function Thumb({ media, item }) {
    const src = item.image_url || media?.thumb_url;
    return (
        <div className="relative aspect-video w-24 shrink-0 overflow-hidden rounded-lg bg-white/5 sm:w-28">
            {src ? (
                <img src={src} alt="" className="h-full w-full object-cover" loading="lazy" />
            ) : (
                <div className="grid h-full place-items-center text-faint">
                    <ListMusic size={18} />
                </div>
            )}
        </div>
    );
}

function SongRow({ item, index, media, mediaProgress, clipProgress, store, onEdit, onDragEnd }) {
    const controls = useDragControls();
    const { toast, confirm } = useFeedback();
    const state = songState(item, media);
    const { playing, progress } = usePreview(item.id);

    const patch = async (body) => {
        try {
            const it = await api.patch(`/api/items/${item.id}`, body);
            store.dispatch({ type: 'item', item: it });
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const retry = async () => {
        try {
            await api.post(`/api/items/${item.id}/retry`);
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const remove = async () => {
        const ok = await confirm({
            title: 'Видалити пісню?',
            message: `«${item.answer || media?.title || 'Без назви'}» зникне з категорії разом з балами команд за неї.`,
            confirmLabel: 'Видалити',
            danger: true,
        });
        if (!ok) return;
        try {
            await api.del(`/api/items/${item.id}`);
            store.reload();
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const busy = state.key === 'downloading' || state.key === 'rendering';
    const p = state.key === 'downloading' ? mediaProgress : state.key === 'rendering' ? clipProgress : null;

    return (
        <Reorder.Item
            value={item}
            dragListener={false}
            dragControls={controls}
            onDragEnd={onDragEnd}
            className="panel group flex items-center gap-2 p-2 pr-3 sm:gap-3"
        >
            <span
                className="cursor-grab touch-none text-faint active:cursor-grabbing"
                onPointerDown={(e) => controls.start(e)}
                title="Перетягніть, щоб змінити порядок"
            >
                <GripVertical size={16} />
            </span>
            <span className="w-5 text-center font-display text-sm font-bold text-faint">{index + 1}</span>
            <button type="button" className="shrink-0" onClick={onEdit} title="Налаштувати">
                <Thumb media={media} item={item} />
            </button>

            <div className="min-w-0 flex-1">
                <InlineEdit
                    value={item.answer}
                    placeholder={media?.title ? 'Відповідь…' : 'Чекаємо назву з YouTube…'}
                    onSave={(answer) => patch({ answer })}
                    className="block w-full font-semibold"
                />
                <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-dim">
                    <span className="font-mono">
                        {fmtTime(item.start)}–{fmtTime(item.end)}
                    </span>
                    {media?.title && <span className="max-w-[26ch] truncate text-faint">{media.title}</span>}
                    <AnimatePresence initial={false}>
                        {state.key !== 'ready' && (
                            <motion.span initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                                <Chip tone={state.tone} icon={busy ? undefined : state.tone === 'bad' ? CircleAlert : undefined}>
                                    {busy && <Spinner size={11} />}
                                    {state.label}
                                    {p != null && ` ${Math.round(p * 100)}%`}
                                </Chip>
                            </motion.span>
                        )}
                    </AnimatePresence>
                </div>
                {state.key === 'download_error' && (
                    <div className="mt-1.5 line-clamp-2 text-xs text-bad" title={media?.error}>
                        {media?.error?.split('\n')[0]}
                    </div>
                )}
                {busy && p != null && <ProgressBar value={p} className="mt-1.5 max-w-xs" />}
            </div>

            <div className="flex shrink-0 items-center gap-0.5">
                {state.key === 'download_error' && (
                    <Button size="sm" variant="ghost" icon={RefreshCw} onClick={retry} title="Спробувати ще раз" />
                )}
                <Button
                    size="sm"
                    variant="ghost"
                    icon={item.show_video ? Eye : EyeOff}
                    className={item.show_video ? 'text-accent-2' : 'text-faint'}
                    onClick={() => patch({ show_video: !item.show_video })}
                    title={item.show_video ? 'Глядачі бачать відео' : 'Лише звук (відео приховано)'}
                />
                <Button
                    size="sm"
                    variant="ghost"
                    icon={playing ? Pause : Play}
                    disabled={!item.clip.url}
                    onClick={() => togglePreview(item.id, item.clip.url)}
                    title="Прослухати фрагмент"
                    style={playing ? { background: `conic-gradient(rgb(255 90 122 / .35) ${progress * 360}deg, transparent 0)` } : undefined}
                />
                <Button size="sm" variant="ghost" icon={Pencil} onClick={onEdit} title="Налаштувати фрагмент" />
                <Button
                    size="sm"
                    variant="ghost"
                    icon={Trash2}
                    onClick={remove}
                    title="Видалити пісню"
                    className="text-faint hover:text-bad"
                />
            </div>
        </Reorder.Item>
    );
}
