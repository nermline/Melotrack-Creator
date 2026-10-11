import { useEffect, useMemo, useState } from 'react';
import { Check, CircleAlert, ExternalLink, Eye, Gamepad2, Monitor, Server, Smartphone, TriangleAlert } from 'lucide-react';
import { api } from '../lib/api';
import { categories, songs, teams } from '../lib/format';
import { QR } from '../ui';
import { cx } from '../lib/cx';
import { useFeedback } from '../ui/feedbackContext';
import { THEME_LIST } from '../show/themes';
import Stage from '../show/Stage';
import { allItems, songState } from './useProjectStore';

function Check2({ ok, warn, children }) {
    const Icon = ok ? Check : warn ? TriangleAlert : CircleAlert;
    return (
        <li className="flex items-start gap-2.5 py-1.5 text-sm">
            <Icon size={17} className={cx('mt-px shrink-0', ok ? 'text-ok' : warn ? 'text-warn' : 'text-bad')} />
            <span>{children}</span>
        </li>
    );
}

// A static welcome slide used to preview the themes.
function sampleView(project, theme) {
    return {
        seq: 0,
        server_now: Date.now(),
        phase: 'category',
        title: project.title,
        theme,
        cat_index: 0,
        cat_count: Math.max(project.categories.length, 1),
        item_index: 0,
        item_count: 0,
        timer: { duration_ms: 0 },
        category: { id: 0, title: project.categories[0]?.title || 'Хіти 2000-х', items: [] },
        revealed: 0,
    };
}

const ROLE = { download: 'завантаження', render: 'нарізка' };

// Workers lists the remote servers that download or cut videos, one line per
// address (one worker often does both).
function Workers({ list }) {
    if (!list?.length) return null;
    const byUrl = new Map();
    for (const w of list) {
        const prev = byUrl.get(w.url);
        byUrl.set(w.url, prev ? { ...prev, roles: [...prev.roles, ROLE[w.role]] } : { ...w, roles: [ROLE[w.role]] });
    }
    return (
        <ul className="m-0 mt-2 list-none p-0">
            {[...byUrl.values()].map((w) => (
                <li key={w.url} className="flex items-start gap-2 py-0.5" title={w.error || w.url}>
                    <Server size={14} className={cx('mt-0.5 shrink-0', w.online ? 'text-ok' : 'text-bad')} />
                    <span className="min-w-0">
                        <span className="text-ink">{w.name || w.url}</span> · {w.roles.join(', ')}
                        {w.online ? (w.ytdlp_version ? ` · yt-dlp ${w.ytdlp_version}` : '') : ' · недоступний'}
                    </span>
                </li>
            ))}
        </ul>
    );
}

export default function ShowTab({ store }) {
    const { project } = store;
    const { toast } = useFeedback();
    const [system, setSystem] = useState(null);
    const [thinkDraft, setThink] = useState(null);
    const think = thinkDraft ?? project.think_seconds;

    // The queue changes as songs are added elsewhere, so keep it fresh.
    useEffect(() => {
        const load = () => api.get('/api/system').then(setSystem).catch(() => {});
        load();
        const id = setInterval(load, 5000);
        return () => clearInterval(id);
    }, []);

    const patch = async (body) => {
        try {
            const p = await api.patch(`/api/projects/${project.id}`, body);
            store.dispatch({ type: 'patch', patch: { theme: p.theme, think_seconds: p.think_seconds } });
            setThink(null);
        } catch (e) {
            toast(e.message, 'bad');
        }
    };

    const checks = useMemo(() => {
        const items = allItems(project);
        const states = items.map((it) => songState(it, project.media[it.media_id]).key);
        return {
            items: items.length,
            broken: states.filter((s) => s === 'download_error' || s === 'clip_error').length,
            busy: states.filter((s) => !['ready', 'download_error', 'clip_error', 'unconfigured'].includes(s)).length,
            todo: states.filter((s) => s === 'unconfigured').length,
            noAnswer: items.filter((i) => !i.answer).length,
            emptyCats: project.categories.filter((c) => c.items.length === 0).length,
            noCriteria: project.categories.filter((c) => !c.criteria?.trim()).map((c) => c.title),
        };
    }, [project]);

    const base = `${location.origin}/p/${project.id}`;
    const devices = [
        { href: `${base}/screen`, icon: Monitor, title: 'Екран', text: 'Відкрийте на компʼютері, підключеному до проєктора, і розгорніть на весь екран.' },
        { href: `${base}/remote`, icon: Smartphone, title: 'Пульт', text: 'Керування показом з телефона. Відскануйте QR і увійдіть тим самим паролем.', qr: true },
        { href: `${base}/preview`, icon: Eye, title: 'Репетиція', text: 'Екран і пульт на одній сторінці. Бачите лише ви — справжній показ не зміниться.' },
    ];

    return (
        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_380px]">
            <div className="flex min-w-0 flex-col gap-5">
                <section className="grid gap-3 md:grid-cols-3">
                    {devices.map((d) => (
                        <a
                            key={d.title}
                            href={d.href}
                            target="_blank"
                            rel="noreferrer"
                            className="panel group flex flex-col p-5 text-ink no-underline transition-colors hover:border-accent/50"
                        >
                            <div className="flex items-center justify-between">
                                <d.icon size={22} className="text-accent" />
                                <ExternalLink size={15} className="text-faint group-hover:text-ink" />
                            </div>
                            <div className="mt-3 font-display text-lg font-bold">{d.title}</div>
                            <p className="mt-1 mb-0 flex-1 text-sm text-dim">{d.text}</p>
                            {d.qr && <QR value={d.href} size={120} className="mt-4" />}
                        </a>
                    ))}
                </section>

                <section className="panel p-5">
                    <div className="mb-1 font-bold">Оформлення екрана</div>
                    <p className="mt-0 mb-4 text-sm text-dim">Тему можна змінити будь-коли — екран оновиться одразу.</p>
                    <div className="grid gap-4 md:grid-cols-3">
                        {THEME_LIST.map((t) => (
                            <button
                                key={t.id}
                                type="button"
                                onClick={() => patch({ theme: t.id })}
                                className={cx(
                                    'overflow-hidden rounded-2xl border-2 bg-panel-2 text-left transition-colors',
                                    project.theme === t.id ? 'border-accent' : 'border-transparent hover:border-line-strong',
                                )}
                            >
                                <div className="pointer-events-none relative aspect-video">
                                    <Stage view={sampleView(project, t.id)} offset={0} preview />
                                </div>
                                <div className="p-3">
                                    <div className="flex items-center gap-2 font-bold">
                                        {t.name}
                                        {project.theme === t.id && <Check size={16} className="text-accent" />}
                                    </div>
                                    <div className="mt-0.5 text-xs text-dim">{t.description}</div>
                                </div>
                            </button>
                        ))}
                    </div>
                </section>

                <section className="panel p-5">
                    <div className="mb-3 font-bold">Таймінг</div>
                    <label className="flex flex-wrap items-center gap-3 text-sm">
                        <span className="text-dim">Час на роздуми після кожної пісні</span>
                        <input
                            type="range"
                            min={3}
                            max={60}
                            value={think}
                            onChange={(e) => setThink(Number(e.target.value))}
                            onPointerUp={() => think !== project.think_seconds && patch({ think_seconds: think })}
                            onKeyUp={() => think !== project.think_seconds && patch({ think_seconds: think })}
                            className="w-48"
                        />
                        <span className="w-12 font-mono font-bold">{think} с</span>
                    </label>
                </section>
            </div>

            <aside className="flex flex-col gap-5">
                <section className="panel p-5">
                    <div className="mb-2 flex items-center gap-2 font-bold">
                        <Gamepad2 size={18} /> Готовність
                    </div>
                    <ul className="m-0 list-none p-0">
                        <Check2 ok={project.categories.length > 0 && checks.emptyCats === 0} warn={checks.emptyCats > 0}>
                            {categories(project.categories.length)}
                            {checks.emptyCats > 0 && `, з них порожніх: ${checks.emptyCats}`}
                        </Check2>
                        <Check2
                            ok={checks.items > 0 && checks.broken === 0 && checks.busy === 0 && checks.todo === 0}
                            warn={(checks.busy > 0 || checks.todo > 0) && checks.broken === 0}
                        >
                            {checks.items === 0
                                ? 'Пісень ще немає'
                                : checks.broken > 0
                                  ? `${songs(checks.broken)} з помилкою — відкрийте «Програму»`
                                  : checks.busy > 0
                                    ? `Готуються: ${songs(checks.busy)}`
                                    : checks.todo > 0
                                      ? `Готово ${checks.items - checks.todo} з ${songs(checks.items)}`
                                      : `Усі ${songs(checks.items)} готові`}
                        </Check2>
                        {checks.todo > 0 && (
                            <Check2 warn>Без вибраного фрагмента: {songs(checks.todo)} — відкрийте їх у «Програмі» й збережіть</Check2>
                        )}
                        {project.categories.length > 0 && (
                            <Check2 ok={checks.noCriteria.length === 0} warn>
                                {checks.noCriteria.length === 0
                                    ? 'Критерії оцінювання заповнені'
                                    : `Без критеріїв оцінювання: ${checks.noCriteria.map((t) => `«${t}»`).join(', ')}`}
                            </Check2>
                        )}
                        <Check2 ok={checks.noAnswer === 0} warn>
                            {checks.noAnswer === 0 ? 'Відповіді заповнені' : `Без відповіді: ${songs(checks.noAnswer)}`}
                        </Check2>
                        <Check2 ok={project.teams.length > 0} warn>
                            {project.teams.length > 0 ? `${teams(project.teams.length)}` : 'Команд ще немає'}
                        </Check2>
                    </ul>
                    {system && (
                        <div className="mt-2 border-t border-line pt-3 text-sm text-dim">
                            <p className="m-0">
                                Черга: завантажень {system.queue.downloads}, нарізок {system.queue.renders} · кліпи {system.clip_height}p
                            </p>
                            <Workers list={system.workers} />
                        </div>
                    )}
                </section>
            </aside>
        </div>
    );
}
