import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ChevronDown, ClipboardCheck, ListChecks, Trophy, UserRound, Users } from 'lucide-react';
import { api } from '../lib/api';
import { fmtPoints } from '../lib/format';
import { Button, Empty } from '../ui';
import { cx } from '../lib/cx';
import { useFeedback } from '../ui/feedbackContext';

// Each category has its own rules, so points are typed in by hand: any number
// from 0 up, fractions included. An empty cell means "not checked yet".
const toInput = (v) => (v === undefined ? '' : String(v).replace('.', ','));

function parsePoints(text) {
    const t = text.trim().replace(',', '.');
    if (t === '') return null;
    const n = Number(t);
    return Number.isFinite(n) && n >= 0 ? Math.round(n * 100) / 100 : undefined;
}

function cellTone(v) {
    if (v === undefined) return 'border-dashed border-line-strong bg-transparent';
    if (v > 0) return 'border-ok/40 bg-ok/15 text-ok';
    return 'border-line bg-white/[0.05] text-dim';
}

// PointsCell edits one team's points for one song. While it has focus it keeps
// its own draft, so live updates from other checkers do not overwrite typing.
function PointsCell({ value, row, col, title, onCommit, onMove }) {
    const [draft, setDraft] = useState(null);
    const { toast } = useFeedback();

    const commit = () => {
        if (draft === null) return;
        const unchanged = draft === toInput(value);
        const parsed = parsePoints(draft);
        setDraft(null);
        if (unchanged) return;
        if (parsed === undefined) {
            toast(`«${draft}» — не число`, 'bad');
            return;
        }
        if (parsed === null ? value !== undefined : parsed !== value) onCommit(parsed);
    };

    return (
        <input
            data-cell={`${row}:${col}`}
            inputMode="decimal"
            autoComplete="off"
            title={title}
            value={draft ?? toInput(value)}
            onFocus={(e) => {
                setDraft(toInput(value));
                e.target.select();
            }}
            onChange={(e) => setDraft(e.target.value.replace(/[^0-9.,]/g, ''))}
            onBlur={commit}
            onKeyDown={(e) => {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    onMove(row, col, e.shiftKey ? 'prev' : 'next');
                } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                    e.preventDefault();
                    onMove(row, col, e.key === 'ArrowDown' ? 'down' : 'up');
                } else if (e.key === 'Escape') {
                    setDraft(null);
                    requestAnimationFrame(() => e.target.blur());
                }
            }}
            className={cx(
                'h-9 w-14 rounded-lg border text-center text-base font-bold tabular-nums outline-none transition-colors',
                'focus:border-accent focus:bg-white/[0.08] focus:text-ink focus:ring-2 focus:ring-accent/30',
                cellTone(value),
            )}
        />
    );
}

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

    const fillZeros = (team) => {
        for (const it of cat.items) if (scores?.[`${team.id}:${it.id}`] === undefined) put(team, it, 0);
    };

    // Enter walks a team's sheet left to right, then on to the next team.
    const move = (row, col, dir) => {
        const rows = project.teams.length;
        const cols = cat.items.length;
        let r;
        let c = col;
        if (dir === 'down' || dir === 'up') {
            r = Math.min(Math.max(row + (dir === 'down' ? 1 : -1), 0), rows - 1);
        } else {
            const flat = row * cols + col + (dir === 'next' ? 1 : -1);
            if (flat < 0 || flat >= rows * cols) {
                document.activeElement?.blur();
                return;
            }
            r = Math.floor(flat / cols);
            c = flat % cols;
        }
        document.querySelector(`[data-cell="${r}:${c}"]`)?.focus();
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
                    <div className="min-w-0">
                        <div className="font-bold">{cat.title}</div>
                        {cat.responsible && (
                            <div className="mt-0.5 flex items-center gap-1.5 text-xs text-dim">
                                <UserRound size={12} /> {cat.responsible}
                            </div>
                        )}
                    </div>
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
                                    <th
                                        key={it.id}
                                        className="px-1 py-2 text-center font-semibold"
                                        title={it.alt_answer ? `${it.answer}\nабо: ${it.alt_answer}` : it.answer}
                                    >
                                        {i + 1}
                                    </th>
                                ))}
                                <th className="px-3 py-2 text-right font-semibold" title="Бали за цю категорію">Σ</th>
                                <th className="px-2 py-2" />
                            </tr>
                        </thead>
                        <tbody>
                            {project.teams.map((t, row) => {
                                let sum = 0;
                                let complete = true;
                                return (
                                    <tr key={t.id} className="group">
                                        <td className="sticky left-0 z-10 max-w-[180px] truncate border-t border-line bg-panel px-4 py-1.5 font-semibold">
                                            {t.name}
                                        </td>
                                        {cat.items.map((it, col) => {
                                            const v = scores?.[`${t.id}:${it.id}`];
                                            if (v === undefined) complete = false;
                                            else sum += v;
                                            return (
                                                <td key={it.id} className="border-t border-line px-0.5 py-1.5 text-center">
                                                    <PointsCell
                                                        value={v}
                                                        row={row}
                                                        col={col}
                                                        title={`${t.name} · ${col + 1}. ${it.answer || 'пісня'}${it.alt_answer ? ` (або: ${it.alt_answer})` : ''}`}
                                                        onCommit={(points) => put(t, it, points)}
                                                        onMove={move}
                                                    />
                                                </td>
                                            );
                                        })}
                                        <td className="border-t border-line px-3 text-right">
                                            <span className={cx('inline-block w-10 font-extrabold tabular-nums', complete ? 'text-ink' : 'text-dim')}>
                                                {fmtPoints(sum)}
                                            </span>
                                        </td>
                                        <td className="border-t border-line px-2 text-right">
                                            <Button
                                                size="sm"
                                                variant="ghost"
                                                className={cx('text-faint', complete && 'invisible')}
                                                onClick={() => fillZeros(t)}
                                                title="Поставити 0 у ще порожні клітинки"
                                                tabIndex={-1}
                                            >
                                                решта 0
                                            </Button>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            </div>

            <div className="flex flex-col gap-4 self-start">
                {cat.criteria ? (
                    <div className="panel px-4 py-3">
                        <div className="mb-1.5 flex items-center gap-1.5 font-bold">
                            <ListChecks size={16} className="text-accent-2" /> Критерії оцінювання
                        </div>
                        <p className="m-0 text-sm whitespace-pre-line text-dim">{cat.criteria}</p>
                    </div>
                ) : (
                    <p className="m-0 px-1 text-xs text-faint">
                        Критерії оцінювання цієї категорії можна записати у «Програмі» — тут їх побачать усі, хто рахує бали.
                    </p>
                )}
                <div className="panel">
                    <button type="button" onClick={() => setShowKey((v) => !v)} className="flex w-full items-center justify-between px-4 py-3 font-bold">
                        Відповіді
                        <ChevronDown size={16} className={cx('transition-transform', showKey && 'rotate-180')} />
                    </button>
                    {showKey && (
                        <ol className="m-0 list-none border-t border-line p-0">
                            {cat.items.map((it, i) => (
                                <li key={it.id} className="flex gap-3 border-b border-line px-4 py-2 text-sm last:border-b-0">
                                    <span className="w-5 shrink-0 text-right font-display font-bold text-faint">{i + 1}</span>
                                    <span className="min-w-0">
                                        <span className={it.answer ? '' : 'text-faint italic'}>{it.answer || 'відповідь не вказана'}</span>
                                        {it.alt_answer && <span className="block text-xs text-dim">або: {it.alt_answer}</span>}
                                    </span>
                                </li>
                            ))}
                        </ol>
                    )}
                </div>
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
            return { id: t.id, name: t.name, perCat, total: perCat.reduce((a, b) => a + b, 0) };
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
                            <td className="border-t border-line px-4 text-right text-base font-extrabold tabular-nums">{fmtPoints(r.total)}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}
