import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { AnimatePresence, motion, Reorder, useDragControls } from 'framer-motion';
import {
    CircleAlert,
    CircleDashed,
    Eye,
    EyeOff,
    FileAudio,
    GripVertical,
    ListChecks,
    ListMusic,
    MonitorPlay,
    Pause,
    Pencil,
    Play,
    Plus,
    RefreshCw,
    Trash2,
    Undo2,
    UserRound,
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
    const openEditor = (id) => {
        stopPreview();
        setEditing(id);
    };

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
                        index={project.categories.findIndex((c) => c.id === selected.id)}
                        store={store}
                        onRename={(t) => renameCategory(selected, t)}
                        onDelete={() => deleteCategory(selected)}
                        onEdit={openEditor}
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
    let todo = 0;
    for (const it of cat.items) {
        const s = songState(it, media[it.media_id]).key;
        if (s === 'download_error' || s === 'clip_error') bad++;
        else if (s === 'unconfigured') todo++;
        else if (s !== 'ready') busy++;
    }
    return { bad, busy, todo };
}

// startDrag begins dragging a row by its handle without selecting the text
// of the rows the pointer passes over.
function startDrag(e, controls) {
    e.preventDefault();
    window.getSelection()?.removeAllRanges();
    document.body.classList.add('select-none');
    const done = () => {
        document.body.classList.remove('select-none');
        window.removeEventListener('pointerup', done);
        window.removeEventListener('pointercancel', done);
    };
    window.addEventListener('pointerup', done);
    window.addEventListener('pointercancel', done);
    controls.start(e);
}

function CategoryRow({ cat, index, media, active, onSelect, onDragEnd }) {
    const controls = useDragControls();
    const { bad, busy, todo } = readiness(cat, media);
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
                onPointerDown={(e) => startDrag(e, controls)}
                onClick={(e) => e.stopPropagation()}
                title="Перетягніть, щоб змінити порядок"
            >
                <GripVertical size={16} />
            </span>
            <span className="w-4 text-center text-xs font-bold text-faint">{index + 1}</span>
            <span className="min-w-0 flex-1 truncate text-sm font-semibold">{cat.title}</span>
            {bad > 0 && <CircleAlert size={15} className="text-bad" title="Є пісні з помилками" />}
            {bad === 0 && busy > 0 && <Spinner size={13} className="text-info" />}
            {bad === 0 && busy === 0 && todo > 0 && (
                <CircleDashed size={15} className="text-warn" title="Є пісні без вибраного фрагмента" />
            )}
            <span className="text-xs text-faint">{cat.items.length}</span>
        </Reorder.Item>
    );
}

function CategoryView({ cat, index, store, onRename, onDelete, onEdit }) {
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
                <div className="flex flex-wrap gap-2">
                    <a
                        href={`/p/${project.id}/preview?cat=${index}`}
                        target="_blank"
                        rel="noreferrer"
                        className="btn btn-sm no-underline"
                        title="Відкрити репетицію з цієї категорії. Справжній показ не зміниться"
                    >
                        <MonitorPlay size={15} strokeWidth={2.2} /> Переглянути категорію
                    </a>
                    <Button variant="danger" size="sm" icon={Trash2} onClick={onDelete}>
                        Видалити категорію
                    </Button>
                </div>
            </div>

            <CategoryDetails cat={cat} store={store} />

            <AddSongs cat={cat} store={store} onAdded={onEdit} />

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

// DraftField edits a text and saves it when the field loses focus. While it
// is not being edited it shows the saved value (changes by others included).
function DraftField({ value, onSave, multiline, className, ...props }) {
    const [draft, setDraft] = useState(null);
    const cancelled = useRef(false);
    const shown = draft ?? value ?? '';
    const Tag = multiline ? 'textarea' : 'input';
    const lines = multiline ? shown.split('\n').length : 1;
    return (
        <Tag
            className={cx('field', multiline && 'resize-y', className)}
            value={shown}
            rows={multiline ? Math.min(Math.max(lines, 2), 8) : undefined}
            onFocus={() => setDraft(value || '')}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={() => {
                if (!cancelled.current && draft !== null && draft.trim() !== (value || '').trim()) onSave(draft.trim());
                cancelled.current = false;
                setDraft(null);
            }}
            onKeyDown={(e) => {
                if (e.key === 'Escape') {
                    cancelled.current = true;
                    e.currentTarget.blur();
                } else if (e.key === 'Enter' && (!multiline || e.metaKey || e.ctrlKey)) {
                    e.preventDefault();
                    e.currentTarget.blur();
                }
            }}
            {...props}
        />
    );
}

// Who prepares the category and how its answers are scored: both are shown
// to the scorers on the "Оцінювання" tab.
function CategoryDetails({ cat, store }) {
    const { toast } = useFeedback();
    const save = async (body) => {
        try {
            await api.patch(`/api/categories/${cat.id}`, body);
            store.reload();
        } catch (err) {
            toast(err.message, 'bad');
        }
    };
    return (
        <div className="panel mb-3 grid gap-3 p-3 sm:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]">
            <label className="block min-w-0">
                <span className="mb-1 flex items-center gap-1.5 text-xs font-semibold text-dim">
                    <UserRound size={13} /> Відповідальні
                </span>
                <DraftField
                    value={cat.responsible}
                    onSave={(responsible) => save({ responsible })}
                    maxLength={80}
                    placeholder="Хто готує категорію"
                    className="py-2 text-sm"
                />
            </label>
            <label className="block min-w-0">
                <span className="mb-1 flex items-center gap-1.5 text-xs font-semibold text-dim">
                    <ListChecks size={13} /> Критерії оцінювання
                </span>
                <DraftField
                    multiline
                    value={cat.criteria}
                    onSave={(criteria) => save({ criteria })}
                    maxLength={2000}
                    placeholder="Напр.: 1 бал за виконавця, 1 бал за назву, 0,5 — якщо майже вгадали"
                    className="py-2 text-sm"
                />
            </label>
        </div>
    );
}

function AddSongs({ cat, store, onAdded }) {
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
            const skipped = res.invalid.length ? `\nПропущено рядків без посилання: ${res.invalid.length}` : '';
            if (n === 1) {
                // One song: pick its fragment right away while the video downloads.
                onAdded(res.items[0].id);
                if (skipped) toast(skipped.trim(), 'info');
            } else if (n > 1) {
                toast(`Додано ${songs(n)}. Відкрийте кожну й виберіть фрагмент — кліп наріжеться після збереження.${skipped}`, 'ok');
            } else {
                toast(skipped.trim() || 'Посилань не знайдено', 'info');
            }
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
            const it = await upload(`/api/categories/${cat.id}/items/upload`, fd, setUploading);
            await store.reload();
            if (it?.id) onAdded(it.id);
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
                    // Pasting a single link into an empty box adds it and opens the editor.
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
                onPointerDown={(e) => startDrag(e, controls)}
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
                    {item.reverse && (
                        <span className="inline-flex items-center gap-1 font-semibold text-accent-2">
                            <Undo2 size={12} /> реверс
                        </span>
                    )}
                    {media?.title && <span className="max-w-[26ch] truncate text-faint">{media.title}</span>}
                    <AnimatePresence initial={false}>
                        {state.key !== 'ready' && (
                            <motion.span
                                initial={{ opacity: 0 }}
                                animate={{ opacity: 1 }}
                                exit={{ opacity: 0 }}
                                onClick={state.key === 'unconfigured' ? onEdit : undefined}
                                className={state.key === 'unconfigured' ? 'cursor-pointer' : undefined}
                            >
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
                    disabled={item.reverse}
                    title={
                        item.reverse
                            ? 'Реверс звучить без відео'
                            : item.show_video
                              ? 'Глядачі бачать відео'
                              : 'Лише звук (відео приховано)'
                    }
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
