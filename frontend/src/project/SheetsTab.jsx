import { useEffect, useMemo, useState } from 'react';
import { FileText, Printer } from 'lucide-react';
import { Button, Empty } from '../ui';
import { cx } from '../lib/cx';
import { songs, teams } from '../lib/format';
import Sheet from './Sheet';

const A4_WIDTH_PX = 793.7; // 210 mm at 96 dpi
const PREVIEW_WIDTH = 280;

// Sheet settings are a per-browser convenience: whoever prints keeps them.
function loadSettings(pid) {
    try {
        return JSON.parse(localStorage.getItem(`melotrack:sheets:${pid}`)) || {};
    } catch {
        return {};
    }
}

// SheetsTab previews the teams' answer sheets (one per category) and opens
// them for printing.
export default function SheetsTab({ store }) {
    const { project } = store;
    const saved = useMemo(() => loadSettings(project.id), [project.id]);
    const [header, setHeader] = useState(saved.header ?? 'Мелотрек');
    const [skip, setSkip] = useState(() => new Set(saved.skip || []));
    useEffect(() => {
        try {
            localStorage.setItem(`melotrack:sheets:${project.id}`, JSON.stringify({ header, skip: [...skip] }));
        } catch {
            /* storage unavailable: settings last until the page closes */
        }
    }, [project.id, header, skip]);

    const cats = project.categories.map((c, i) => ({ ...c, number: i + 1 }));
    const chosen = cats.filter((c) => c.items.length > 0 && !skip.has(c.id));
    const toggle = (id) =>
        setSkip((s) => {
            const next = new Set(s);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
        });
    const print = () => {
        const q = new URLSearchParams({ header: header.trim(), cats: chosen.map((c) => c.id).join(',') });
        window.open(`/p/${project.id}/sheets/print?${q}`, '_blank');
    };

    if (cats.length === 0) {
        return (
            <div className="panel">
                <Empty icon={FileText} title="Спершу додайте категорії">
                    Бланк — це аркуш на кожну категорію з рядками для відповідей.
                </Empty>
            </div>
        );
    }

    const scale = PREVIEW_WIDTH / A4_WIDTH_PX;
    return (
        <div className="grid gap-5 lg:grid-cols-[320px_minmax(0,1fr)]">
            <aside className="flex flex-col gap-4 lg:sticky lg:top-[120px] lg:self-start">
                <section className="panel p-4">
                    <label className="block">
                        <span className="label">Назва на бланку</span>
                        <input className="field" maxLength={60} value={header} onChange={(e) => setHeader(e.target.value)} />
                        <span className="mt-1.5 block text-xs text-faint">Стоїть перед «Тур №…». Можна лишити порожньою.</span>
                    </label>
                    <Button variant="primary" icon={Printer} className="mt-4 w-full" disabled={chosen.length === 0} onClick={print}>
                        Друкувати {chosen.length > 0 && `(${chosen.length})`}
                    </Button>
                    <p className="mt-3 mb-0 text-xs text-dim">
                        Відкриється сторінка друку. Кількість копій ({teams(project.teams.length)}) і «Зберегти як PDF» — у вікні
                        друку браузера.
                    </p>
                </section>
                <section className="panel p-2">
                    <div className="px-2 pt-1 pb-2 text-sm font-bold">Категорії</div>
                    <ul className="m-0 list-none p-0">
                        {cats.map((c) => {
                            const empty = c.items.length === 0;
                            return (
                                <li key={c.id}>
                                    <label
                                        className={cx(
                                            'flex items-center gap-3 rounded-lg px-2 py-2 text-sm',
                                            empty ? 'text-faint' : 'cursor-pointer hover:bg-white/5',
                                        )}
                                    >
                                        <input type="checkbox" disabled={empty} checked={!empty && !skip.has(c.id)} onChange={() => toggle(c.id)} />
                                        <span className="w-5 text-right font-display font-bold text-faint">{c.number}</span>
                                        <span className="min-w-0 flex-1 truncate">{c.title}</span>
                                        <span className="shrink-0 text-xs text-faint">{empty ? 'немає пісень' : songs(c.items.length)}</span>
                                    </label>
                                </li>
                            );
                        })}
                    </ul>
                </section>
            </aside>

            <section className="min-w-0">
                {chosen.length === 0 ? (
                    <div className="panel">
                        <Empty icon={FileText} title="Нічого не вибрано">
                            Позначте категорії ліворуч.
                        </Empty>
                    </div>
                ) : (
                    <div className="grid justify-center gap-5" style={{ gridTemplateColumns: `repeat(auto-fill, ${PREVIEW_WIDTH}px)` }}>
                        {chosen.map((c) => (
                            <div
                                key={c.id}
                                className="overflow-hidden rounded-lg shadow-xl ring-1 ring-black/10"
                                style={{ width: PREVIEW_WIDTH, height: PREVIEW_WIDTH * (297 / 210) }}
                            >
                                <div style={{ transform: `scale(${scale})`, transformOrigin: 'top left' }}>
                                    <Sheet header={header.trim()} number={c.number} category={c} />
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </section>
        </div>
    );
}
