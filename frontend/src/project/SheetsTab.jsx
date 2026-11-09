import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { FileText, Printer } from 'lucide-react';
import { Button, Empty } from '../ui';
import { cx } from '../lib/cx';
import { songs } from '../lib/format';
import Sheet, { SHEET_HEADER } from './Sheet';

const A4_WIDTH_PX = 793.7; // 210 mm at 96 dpi

// Which categories to leave out is a per-browser convenience: whoever prints keeps it.
function loadSkipped(pid) {
    try {
        return JSON.parse(localStorage.getItem(`melotrack:sheets:${pid}`))?.skip || [];
    } catch {
        return [];
    }
}

// SheetPreview shows a sheet scaled down to the width of its box.
function SheetPreview({ category, className }) {
    const ref = useRef(null);
    const [scale, setScale] = useState(0);
    useLayoutEffect(() => {
        const el = ref.current;
        const fit = () => setScale(el.clientWidth / A4_WIDTH_PX);
        fit();
        const ro = new ResizeObserver(fit);
        ro.observe(el);
        return () => ro.disconnect();
    }, []);
    return (
        <div ref={ref} className={cx('overflow-hidden rounded-lg shadow-xl ring-1 ring-black/10', className)} style={{ aspectRatio: '210 / 297' }}>
            <div style={{ transform: `scale(${scale})`, transformOrigin: 'top left', visibility: scale ? 'visible' : 'hidden' }}>
                <Sheet header={SHEET_HEADER} number={category.number} category={category} />
            </div>
        </div>
    );
}

// SheetsTab previews the teams' answer sheets (one per category) and opens
// them for printing.
export default function SheetsTab({ store }) {
    const { project } = store;
    const saved = useMemo(() => loadSkipped(project.id), [project.id]);
    const [skip, setSkip] = useState(() => new Set(saved));
    useEffect(() => {
        try {
            localStorage.setItem(`melotrack:sheets:${project.id}`, JSON.stringify({ skip: [...skip] }));
        } catch {
            /* storage unavailable: the choice lasts until the page closes */
        }
    }, [project.id, skip]);

    const cats = project.categories.map((c, i) => ({ ...c, number: i + 1 }));
    const chosen = cats.filter((c) => c.items.length > 0 && !skip.has(c.id));
    const toggle = (id) =>
        setSkip((s) => {
            const next = new Set(s);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
        });
    const print = () => window.open(`/p/${project.id}/sheets/print?cats=${chosen.map((c) => c.id).join(',')}`, '_blank');

    if (cats.length === 0) {
        return (
            <div className="panel">
                <Empty icon={FileText} title="Спершу додайте категорії">
                    Бланк — це аркуш на кожну категорію з рядками для відповідей.
                </Empty>
            </div>
        );
    }

    return (
        <div className="grid gap-5 lg:min-h-0 lg:flex-1 lg:grid-cols-[320px_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)]">
            <aside className="flex flex-col gap-4 lg:min-h-0">
                <Button variant="primary" icon={Printer} className="w-full shrink-0" disabled={chosen.length === 0} onClick={print}>
                    Друкувати {chosen.length > 0 && `(${chosen.length})`}
                </Button>
                <section className="panel flex flex-col p-2 lg:min-h-0">
                    <div className="shrink-0 px-2 pt-1 pb-2 text-sm font-bold">Категорії</div>
                    <ul className="m-0 list-none p-0 lg:min-h-0 lg:overflow-y-auto">
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

            <section className="min-w-0 lg:-mr-2 lg:min-h-0 lg:overflow-y-auto lg:pr-2">
                {chosen.length === 0 ? (
                    <div className="panel">
                        <Empty icon={FileText} title="Нічого не вибрано">
                            Позначте категорії ліворуч.
                        </Empty>
                    </div>
                ) : (
                    // Two sheets per row; a single one left over sits in the middle.
                    <div className="grid gap-5 pb-1 sm:grid-cols-2">
                        {chosen.map((c, i) => (
                            <SheetPreview
                                key={c.id}
                                category={c}
                                className={chosen.length % 2 === 1 && i === chosen.length - 1 ? 'sm:col-span-2 sm:mx-auto sm:w-[calc(50%-0.625rem)]' : ''}
                            />
                        ))}
                    </div>
                )}
            </section>
        </div>
    );
}
