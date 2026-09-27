import { useEffect, useMemo, useRef, useState } from 'react';
import {
    CircleAlert,
    ExternalLink,
    Eye,
    ImagePlus,
    Link as LinkIcon,
    RefreshCw,
    Trash2,
    Upload,
    Wand2,
} from 'lucide-react';
import { api, upload } from '../lib/api';
import { fmtTime } from '../lib/format';
import { Button, Chip, Field, Modal, ProgressBar, Spinner, Toggle } from '../ui';
import { useFeedback } from '../ui/feedbackContext';
import ClipEditor from './ClipEditor';
import ImageCropModal from './ImageCropModal';
import { songState } from './useProjectStore';

function toDraft(item) {
    return {
        answer: item.answer,
        show_video: item.show_video,
        gain_db: item.gain_db,
        start: item.start,
        end: item.end,
        frame: item.frame,
        crop: item.crop,
    };
}

function diff(item, d) {
    const out = {};
    const base = toDraft(item);
    for (const k of Object.keys(d)) {
        if (JSON.stringify(d[k]) !== JSON.stringify(base[k])) out[k] = d[k];
    }
    if (out.crop === null) delete out.crop;
    return out;
}

export default function SongEditor({ open, item, media, store, onClose: close0 }) {
    // The draft starts as the saved song and survives live updates of it
    // (a finished render must not wipe what the editor is changing).
    const [edits, setEdits] = useState({ id: null, draft: null });
    const [saving, setSaving] = useState(false);
    const { toast, confirm } = useFeedback();
    const draft = item && edits.id === item.id ? edits.draft : item ? toDraft(item) : null;
    const setDraft = (d) => setEdits({ id: item.id, draft: d });
    const onClose = () => {
        setEdits({ id: null, draft: null });
        close0();
    };

    const changes = useMemo(() => (item && draft ? diff(item, draft) : {}), [item, draft]);
    const dirty = Object.keys(changes).length > 0;

    const close = async () => {
        if (dirty && !(await confirm({ title: 'Закрити без збереження?', message: 'Зміни в цій пісні буде втрачено.', confirmLabel: 'Закрити', danger: true })))
            return;
        onClose();
    };

    const save = async () => {
        setSaving(true);
        try {
            const it = await api.patch(`/api/items/${item.id}`, changes);
            store.dispatch({ type: 'item', item: it });
            if (changes.start !== undefined || changes.end !== undefined || changes.frame || changes.crop || changes.gain_db !== undefined)
                toast('Збережено. Фрагмент переріжеться за кілька секунд');
            else toast('Збережено');
            onClose();
        } catch (e) {
            toast(e.message, 'bad');
        } finally {
            setSaving(false);
        }
    };

    if (!item || !draft) return <Modal open={false} />;

    return (
        <Modal
            open={open}
            onClose={close}
            width={1120}
            title={item.answer || media?.title || 'Пісня'}
            footer={
                <>
                    <Button variant="ghost" onClick={close}>
                        Скасувати
                    </Button>
                    <Button variant="primary" onClick={save} loading={saving} disabled={!dirty}>
                        Зберегти
                    </Button>
                </>
            }
        >
            <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
                <div className="min-w-0">
                    <SourcePanel item={item} media={media} store={store}>
                        {media?.status === 'ready' && (
                            <ClipEditor media={media} value={draft} onChange={setDraft} />
                        )}
                    </SourcePanel>
                </div>

                <div className="flex flex-col gap-5">
                    <Field label="Правильна відповідь" hint="Так її побачать гравці на слайді відповідей">
                        <input
                            className="field"
                            maxLength={300}
                            value={draft.answer}
                            placeholder="Виконавець — Назва"
                            onChange={(e) => setDraft({ ...draft, answer: e.target.value })}
                        />
                        {media?.suggested && media.suggested !== draft.answer && (
                            <button
                                type="button"
                                className="mt-1.5 inline-flex items-center gap-1 text-left text-xs text-info hover:underline"
                                onClick={() => setDraft({ ...draft, answer: media.suggested })}
                                title={`Оригінальна назва: ${media.title}`}
                            >
                                <Wand2 size={12} className="shrink-0" /> З YouTube: {media.suggested}
                            </button>
                        )}
                    </Field>

                    <div>
                        <Toggle
                            checked={draft.show_video}
                            onChange={(v) => setDraft({ ...draft, show_video: v })}
                            label="Показувати відео гравцям"
                        />
                        <p className="mt-1.5 mb-0 text-xs text-faint">
                            Вимкнено — на екрані анімація теми, звучить лише звук.
                        </p>
                    </div>

                    <Field
                        label={`Гучність: ${draft.gain_db > 0 ? '+' : ''}${draft.gain_db} дБ`}
                        hint="Гучність усіх пісень вирівнюється автоматично; тут — лише корекція"
                    >
                        <input
                            type="range"
                            min={-12}
                            max={12}
                            step={0.5}
                            value={draft.gain_db}
                            className="w-full"
                            onChange={(e) => setDraft({ ...draft, gain_db: Number(e.target.value) })}
                            onDoubleClick={() => setDraft({ ...draft, gain_db: 0 })}
                        />
                    </Field>

                    <AnswerImage item={item} media={media} store={store} />
                </div>
            </div>
        </Modal>
    );
}

function SourcePanel({ item, media, store, children }) {
    const [url, setUrl] = useState('');
    const [uploading, setUploading] = useState(null);
    const fileRef = useRef(null);
    const { toast } = useFeedback();
    const state = songState(item, media);
    const progress = store.mediaProgress[item.media_id];
    const clipProgress = store.clipProgress[item.id];

    const replaceUrl = async (e) => {
        e.preventDefault();
        try {
            const it = await api.patch(`/api/items/${item.id}`, { youtube_url: url });
            setUrl('');
            store.dispatch({ type: 'item', item: it });
            store.reload();
            toast('Посилання замінено, завантажую нове відео');
        } catch (err) {
            toast(err.message, 'bad');
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
            await upload(`/api/items/${item.id}/source`, fd, setUploading);
            toast('Файл отримано, обробляю');
        } catch (err) {
            toast(err.message, 'bad');
        } finally {
            setUploading(null);
        }
    };

    const retry = async () => {
        try {
            await api.post(`/api/items/${item.id}/retry`);
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    const rerender = async () => {
        try {
            await api.post(`/api/items/${item.id}/render`);
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    return (
        <div>
            {media?.status !== 'ready' && (
                <div className="grid aspect-video place-items-center rounded-xl bg-black/40 p-6 text-center">
                    {media?.status === 'error' ? (
                        <div className="max-w-md">
                            <CircleAlert size={32} className="mx-auto text-bad" />
                            <p className="mt-3 mb-1 font-semibold whitespace-pre-line">{media.error?.split('\n')[0]}</p>
                            {media.error?.includes('\n') && (
                                <p className="m-0 font-mono text-xs break-words text-faint">{media.error.split('\n').slice(1).join(' ')}</p>
                            )}
                            <div className="mt-4 flex flex-wrap justify-center gap-2">
                                {media.kind === 'youtube' && (
                                    <Button icon={RefreshCw} onClick={retry}>
                                        Спробувати ще раз
                                    </Button>
                                )}
                                <Button variant="primary" icon={Upload} onClick={() => fileRef.current?.click()}>
                                    Завантажити файл вручну
                                </Button>
                            </div>
                        </div>
                    ) : (
                        <div className="w-64">
                            <Spinner size={28} className="mx-auto text-info" />
                            <p className="mt-3 mb-2 text-sm text-dim">
                                {state.label}
                                {progress != null && ` — ${Math.round(progress * 100)}%`}
                            </p>
                            {progress != null && <ProgressBar value={progress} />}
                        </div>
                    )}
                </div>
            )}

            {children}

            <div className="mt-5 grid gap-3 rounded-xl border border-line p-4 text-sm">
                <div className="flex flex-wrap items-center gap-2">
                    <span className="label m-0">Джерело</span>
                    {media?.kind === 'youtube' ? (
                        <a href={media.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-info">
                            youtube.com/watch?v={media.youtube_id} <ExternalLink size={13} />
                        </a>
                    ) : (
                        <span className="text-dim">файл з компʼютера</span>
                    )}
                    {media?.replaced && <Chip tone="info">замінено файлом</Chip>}
                    {media?.duration > 0 && <span className="text-faint">· {fmtTime(media.duration)}</span>}
                    {media?.width > 0 && (
                        <span className="text-faint">
                            · {media.width}×{media.height}
                        </span>
                    )}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                    <span className="label m-0">Фрагмент</span>
                    <Chip tone={state.tone}>
                        {state.key === 'rendering' && <Spinner size={11} />}
                        {state.label}
                        {state.key === 'rendering' && clipProgress != null && ` ${Math.round(clipProgress * 100)}%`}
                    </Chip>
                    {item.clip.error && <span className="text-xs text-bad">{item.clip.error}</span>}
                    {item.clip.url && (
                        <a href={item.clip.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-xs text-info">
                            <Eye size={13} /> відкрити кліп
                        </a>
                    )}
                    {media?.status === 'ready' && (
                        <Button size="sm" variant="ghost" icon={RefreshCw} onClick={rerender}>
                            Перерізати
                        </Button>
                    )}
                </div>
                <form onSubmit={replaceUrl} className="flex gap-2">
                    <div className="relative flex-1">
                        <LinkIcon size={14} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-faint" />
                        <input
                            className="field py-1.5 pl-8"
                            placeholder="Інше посилання на YouTube…"
                            value={url}
                            onChange={(e) => setUrl(e.target.value)}
                        />
                    </div>
                    <Button type="submit" size="sm" disabled={!url.trim()}>
                        Замінити
                    </Button>
                </form>
                <div className="flex flex-wrap items-center gap-2">
                    <input ref={fileRef} type="file" accept="video/*,audio/*" hidden onChange={onFile} />
                    <Button size="sm" variant="ghost" icon={Upload} loading={uploading !== null} onClick={() => fileRef.current?.click()}>
                        {uploading !== null ? `Надсилаю ${Math.round(uploading * 100)}%` : 'Замінити файлом з компʼютера'}
                    </Button>
                    <span className="text-xs text-faint">якщо YouTube не віддає відео серверу</span>
                </div>
            </div>
        </div>
    );
}

function AnswerImage({ item, media, store }) {
    const [src, setSrc] = useState(null);
    const [busy, setBusy] = useState(false);
    const fileRef = useRef(null);
    const { toast } = useFeedback();

    useEffect(() => () => src?.startsWith('blob:') && URL.revokeObjectURL(src), [src]);

    const onFile = (e) => {
        const f = e.target.files?.[0];
        e.target.value = '';
        if (f) setSrc(URL.createObjectURL(f));
    };

    const save = async (blob) => {
        setSrc(null);
        setBusy(true);
        try {
            const fd = new FormData();
            fd.append('image', blob, 'answer.jpg');
            const it = await upload(`/api/items/${item.id}/image`, fd);
            store.dispatch({ type: 'item', item: it });
        } catch (err) {
            toast(err.message, 'bad');
        } finally {
            setBusy(false);
        }
    };

    const remove = async () => {
        try {
            const it = await api.del(`/api/items/${item.id}/image`);
            store.dispatch({ type: 'item', item: it });
        } catch (err) {
            toast(err.message, 'bad');
        }
    };

    return (
        <div>
            <div className="label">Фото на слайді відповіді</div>
            <div className="relative aspect-square w-full max-w-[240px] overflow-hidden rounded-xl bg-white/5">
                {item.answer_image ? (
                    <img src={item.answer_image} alt="" className="h-full w-full object-cover" />
                ) : (
                    <div className="grid h-full place-items-center text-sm text-faint">без фото</div>
                )}
                {busy && (
                    <div className="absolute inset-0 grid place-items-center bg-black/50">
                        <Spinner size={22} />
                    </div>
                )}
                {!item.image_url && item.answer_image && (
                    <span className="absolute bottom-2 left-2 rounded-md bg-black/70 px-2 py-0.5 text-[11px]">прев'ю YouTube</span>
                )}
            </div>
            <div className="mt-2 flex flex-wrap gap-2">
                <input ref={fileRef} type="file" accept="image/*" hidden onChange={onFile} />
                <Button size="sm" icon={ImagePlus} onClick={() => fileRef.current?.click()}>
                    Своє фото
                </Button>
                {media?.thumb_url && (
                    <Button size="sm" variant="ghost" icon={Wand2} onClick={() => setSrc(media.thumb_url)} title="Обрізати прев'ю YouTube до квадрата">
                        З прев'ю
                    </Button>
                )}
                {item.image_url && <Button size="sm" variant="ghost" icon={Trash2} onClick={remove} title="Повернути прев'ю YouTube" />}
            </div>
            <ImageCropModal open={!!src} src={src} onCancel={() => setSrc(null)} onConfirm={save} />
        </div>
    );
}
