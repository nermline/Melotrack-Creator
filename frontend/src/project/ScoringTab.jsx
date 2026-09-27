import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ChevronDown, ClipboardCheck, Minus, Plus, Trophy, Users } from 'lucide-react';
import { api } from '../lib/api';
import { fmtPoints } from '../lib/format';
import { Button, Empty } from '../ui';
import { cx } from '../lib/cx';
import { useFeedback } from '../ui/feedbackContext';

// Tapping a cell cycles: not checked → correct → wrong → half → correct…
const NEXT = { undefined: 1, 1: 0, 0: 0.5, 0.5: 1 };
const CELL = {
    undefined: { label: '·', cls: 'bg-white/[0.04] text-faint border-line' },
    1: { label: '✓', cls: 'bg-ok/20 text-ok border-ok/40' },
    0.5: { label: '½', cls: 'bg-warn/20 text-warn border-warn/40' },
    0: { label: '✗', cls: 'bg-bad/15 text-bad border-bad/30' },
};

function rank(rows) {
    const sorted = [...rows].sort((a, b) => b.total - a.total || a.name.localeCompare(b.name, 'uk'));
    sorted.forEach((r, i) => {
        r.rank = i > 0 && r.total === sorted[i - 1].total ? sorted[i - 1].rank : i + 1;
    });
    return sorted;
}

export default function ScoringTab({ store }) {
    const { project, scores } = store;
    const [params, setParams] = useSearchParams();
    const view = params.get('s') || String(project.categories[0]?.id ?? 'summary');
    const cat = project.categories.find((c) => String(c.id) === view);
    const setView = (v) => setParams({ s: v }, { replace: true });

    if (project.teams.length === 0) {
        return (
            <div className="panel">
                <Empty icon={Users} title="Спершу додайте команди">
                    Оцінювання відкриється, щойно зʼявиться хоча б одна команда (вкладка «Команди»).
                </Empty>
            </div>
        );
    }

    return (
        <div>
            <div className="mb-4 flex gap-1.5 overflow-x-auto pb-1">
                {project.categories.map((c, i) => (
                    <CatTab key={c.id} active={view === String(c.id)} onClick={() => setView(String(c.id))} cat={c} index={i} project={project} scores={scores} />
                ))}
                <button
                    type="button"
                    onClick={() => setView('summary')}
                    className={cx(
                        'inline-flex shrink-0 items-center gap-1.5 rounded-xl border px-3 py-2 text-sm font-semibold',
                        view === 'summary' ? 'border-accent-2/60 bg-accent-2/10 text-accent-2' : 'border-line text-dim hover:text-ink',
                    )}
                >
                    <Trophy size={15} /> Підсумок
                </button>
            </div>
            {cat ? <CategoryScoring key={cat.id} cat={cat} store={store} /> : <Summary project={project} scores={scores} />}
        </div>
    );
}

function checkedTeams(cat, teams, scores) {
    if (!scores || cat.items.length === 0) return 0;
    return teams.filter((t) => cat.items.every((it) => scores[`${t.id}:${it.id}`] !== undefined)).length;
}

function CatTab({ active, onClick, cat, index, project, scores }) {
    const done = checkedTeams(cat, project.teams, scores);
    const all = project.teams.length;
    return (
        <button
            type="button"
            onClick={onClick}
            className={cx(
                'inline-flex shrink-0 items-center gap-2 rounded-xl border px-3 py-2 text-sm font-semibold',
                active ? 'border-accent/60 bg-accent/10 text-ink' : 'border-line text-dim hover:text-ink',
            )}
        >
            <span className="text-faint">{index + 1}</span>
            <span className="max-w-[18ch] truncate">{cat.title}</span>
            <span className={cx('rounded-full px-1.5 text-[11px]', done === all ? 'bg-ok/20 text-ok' : 'bg-white/10 text-faint')}>
                {done}/{all}
            </span>
        </button>
    );
}

function CategoryScoring({ cat, store }) {
    const { project, scores } = store;
    const [showKey, setShowKey] = useState(true);
    const { toast } = useFeedback();

    const put = async (team, item, points) => {
        const key = `${team.id}:${item.id}`;
        const prev = scores?.[key];
        store.dispatch({ type: 'score', team_id: team.id, item_id: item.id, points });
        try {
            await api.put('/api/scores', { team_id: team.id, item_id: item.id, points });
        } catch (e) {
            if (prev === undefined) store.reloadScores();
            else store.dispatch({ type: 'score', team_id: team.id, item_id: item.id, points: prev });
            toast(e.message, 'bad');
        }
    };

    const setBonus = async (team, bonus) => {
        const prev = project.teams;
        store.dispatch({ type: 'teams', teams: prev.map((t) => (t.id === team.id ? { ...t, bonus } : t)) });
        try {
            await api.patch(`/api/teams/${team.id}`, { bonus });
        } catch (e) {
            store.dispatch({ type: 'teams', teams: prev });
            toast(e.message, 'bad');
        }
    };

    const fillWrong = (team) => {
        for (const it of cat.items) if (scores?.[`${team.id}:${it.id}`] === undefined) put(team, it, 0);
    };

    const done = checkedTeams(cat, project.teams, scores);

    if (cat.items.length === 0) {
        return (
            <div className="panel">
                <Empty icon={ClipboardCheck} title="У цій категорії немає пісень" />
            </div>
        );
    }

    return (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
            <div className="panel min-w-0 overflow-hidden">
                <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-3">
                    <div className="font-bold">{cat.title}</div>
                    <div className="text-sm text-dim">
                        Перевірено {done} з {project.teams.length}
                    </div>
                </div>
                <div className="overflow-x-auto">
                    <table className="w-full border-separate border-spacing-0 text-sm">
                        <thead>
                            <tr className="text-xs text-faint">
                                <th className="sticky left-0 z-10 bg-panel px-4 py-2 text-left font-semibold">Команда</th>
                                {cat.items.map((it, i) => (
                                    <th key={it.id} className="px-1 py-2 text-center font-semibold" title={it.answer}>
                                        {i + 1}
                                    </th>
                                ))}
                                <th className="px-3 py-2 text-right font-semibold" title="Бали за цю категорію">Σ</th>
                                <th className="px-2 py-2 text-center font-semibold" title="Бонусні або штрафні бали команди, йдуть у загальний рахунок">
                                    Бонус
                                </th>
                                <th className="px-2 py-2" />
                            </tr>
                        </thead>
                        <tbody>
                            {project.teams.map((t) => {
                                let sum = 0;
                                let complete = true;
                                return (
                                    <tr key={t.id} className="group">
                                        <td className="sticky left-0 z-10 max-w-[180px] truncate border-t border-line bg-panel px-4 py-1.5 font-semibold">
                                            {t.name}
                                        </td>
                                        {cat.items.map((it) => {
                                            const v = scores?.[`${t.id}:${it.id}`];
                                            if (v === undefined) complete = false;
                                            else sum += v;
                                            const c = CELL[v];
                                            return (
                                                <td key={it.id} className="border-t border-line px-0.5 py-1.5 text-center">
                                                    <button
                                                        type="button"
                                                        onClick={() => put(t, it, NEXT[v])}
                                                        onContextMenu={(e) => {
                                                            e.preventDefault();
                                                            put(t, it, 0.5);
                                                        }}
                                                        className={cx('h-9 w-9 rounded-lg border text-base font-bold transition-colors', c.cls)}
                                                        title={`${it.answer || 'пісня'} — натисніть: ✓ → ✗ → ½`}
                                                    >
                                                        {c.label}
                                                    </button>
                                                </td>
                                            );
                                        })}
                                        <td className="border-t border-line px-3 text-right">
                                            <span className={cx('inline-block w-10 font-extrabold tabular-nums', complete ? 'text-ink' : 'text-dim')}>
                                                {fmtPoints(sum)}
                                            </span>
                                        </td>
                                        <td className="border-t border-line px-2">
                                            <div className="flex items-center justify-center gap-0.5">
                                                <Button size="sm" variant="ghost" icon={Minus} onClick={() => setBonus(t, (t.bonus || 0) - 0.5)} aria-label="Мінус пів бала" />
                                                <span className={cx('inline-block w-11 text-center font-mono text-xs tabular-nums', t.bonus ? 'text-accent-2' : 'text-faint')}>
                                                    {t.bonus > 0 ? '+' : ''}
                                                    {fmtPoints(t.bonus || 0)}
                                                </span>
                                                <Button size="sm" variant="ghost" icon={Plus} onClick={() => setBonus(t, (t.bonus || 0) + 0.5)} aria-label="Плюс пів бала" />
                                            </div>
                                        </td>
                                        <td className="border-t border-line px-2 text-right">
                                            <Button
                                                size="sm"
                                                variant="ghost"
                                                className={cx('text-faint', complete && 'invisible')}
                                                onClick={() => fillWrong(t)}
                                                title="Позначити решту як неправильні"
                                                tabIndex={complete ? -1 : 0}
                                            >
                                                решта ✗
                                            </Button>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
                <p className="m-0 border-t border-line px-4 py-2.5 text-xs text-faint">
                    Натискання: ✓ правильно → ✗ неправильно → ½ частково. Правий клік — одразу ½. Бонус — загальний для
                    команди, кроком 0,5.
                </p>
            </div>

            <div className="panel self-start">
                <button type="button" onClick={() => setShowKey((v) => !v)} className="flex w-full items-center justify-between px-4 py-3 font-bold">
                    Відповіді
                    <ChevronDown size={16} className={cx('transition-transform', showKey && 'rotate-180')} />
                </button>
                {showKey && (
                    <ol className="m-0 list-none border-t border-line p-0">
                        {cat.items.map((it, i) => (
                            <li key={it.id} className="flex gap-3 border-b border-line px-4 py-2 text-sm last:border-b-0">
                                <span className="w-5 shrink-0 text-right font-display font-bold text-faint">{i + 1}</span>
                                <span className={it.answer ? '' : 'text-faint italic'}>{it.answer || 'відповідь не вказана'}</span>
                            </li>
                        ))}
                    </ol>
                )}
            </div>
        </div>
    );
}

function Summary({ project, scores }) {
    const rows = useMemo(() => {
        const list = project.teams.map((t) => {
            const perCat = project.categories.map((c) =>
                c.items.reduce((s, it) => s + (scores?.[`${t.id}:${it.id}`] ?? 0), 0),
            );
            return { id: t.id, name: t.name, perCat, bonus: t.bonus || 0, total: perCat.reduce((a, b) => a + b, 0) + (t.bonus || 0) };
        });
        return rank(list);
    }, [project, scores]);

    return (
        <div className="panel overflow-x-auto">
            <table className="w-full border-separate border-spacing-0 text-sm">
                <thead>
                    <tr className="text-xs text-faint">
                        <th className="px-4 py-3 text-left">Місце</th>
                        <th className="px-2 py-3 text-left">Команда</th>
                        {project.categories.map((c, i) => (
                            <th key={c.id} className="px-2 py-3 text-right font-semibold" title={c.title}>
                                {i + 1}. <span className="hidden max-w-[10ch] truncate align-bottom md:inline-block">{c.title}</span>
                            </th>
                        ))}
                        <th className="px-2 py-3 text-right">Бонус</th>
                        <th className="px-4 py-3 text-right">Разом</th>
                    </tr>
                </thead>
                <tbody>
                    {rows.map((r) => (
                        <tr key={r.id}>
                            <td className="border-t border-line px-4 py-2.5">
                                <span
                                    className={cx(
                                        'inline-grid h-7 w-7 place-items-center rounded-full font-display text-xs font-bold',
                                        r.rank === 1 ? 'bg-accent-2 text-bg' : r.rank <= 3 ? 'bg-white/15' : 'text-faint',
                                    )}
                                >
                                    {r.rank}
                                </span>
                            </td>
                            <td className="border-t border-line px-2 font-semibold">{r.name}</td>
                            {r.perCat.map((p, i) => (
                                <td key={i} className="border-t border-line px-2 text-right font-mono text-dim">
                                    {fmtPoints(p)}
                                </td>
                            ))}
                            <td className="border-t border-line px-2 text-right font-mono text-faint">{r.bonus ? fmtPoints(r.bonus) : '—'}</td>
                            <td className="border-t border-line px-4 text-right text-base font-extrabold tabular-nums">{fmtPoints(r.total)}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}
