import { useEffect, useRef, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { ChevronLeft, Printer } from 'lucide-react';
import { api } from '../lib/api';
import { Button, Spinner } from '../ui';
import Sheet from '../project/Sheet';

// The answer sheets alone, one A4 page per category, ready for the browser's
// print dialog (which can also save them as a PDF).
export default function SheetsPrint() {
    const { pid } = useParams();
    const [params] = useSearchParams();
    const [project, setProject] = useState(null);
    const [error, setError] = useState('');

    useEffect(() => {
        api.get(`/api/projects/${pid}`)
            .then(setProject)
            .catch((e) => setError(e.message));
    }, [pid]);

    const header = params.get('header') ?? 'Мелотрек';
    const ids = params.get('cats')?.split(',').filter(Boolean).map(Number);
    const cats = (project?.categories || [])
        .map((c, i) => ({ ...c, number: i + 1 }))
        .filter((c) => (ids ? ids.includes(c.id) : c.items.length > 0));

    useEffect(() => {
        if (project) document.title = `Бланки — ${project.title}`;
    }, [project]);

    // Open the print dialog once, when the sheet font has loaded.
    const printed = useRef(false);
    const ready = cats.length > 0;
    useEffect(() => {
        if (!ready || printed.current) return;
        printed.current = true;
        const go = () => setTimeout(() => window.print(), 300);
        if (document.fonts?.ready) document.fonts.ready.then(go);
        else go();
    }, [ready]);

    if (error) return <p className="p-6 text-bad">{error}</p>;
    if (!project) {
        return (
            <div className="grid min-h-screen place-items-center text-dim">
                <Spinner size={22} />
            </div>
        );
    }
    return (
        <div className="sheets-print">
            <div className="no-print fixed top-3 left-1/2 z-10 flex -translate-x-1/2 items-center gap-2 rounded-2xl bg-panel/95 p-2 shadow-xl ring-1 ring-line backdrop-blur">
                <Link to={`/p/${pid}/sheets`} className="btn btn-sm btn-ghost no-underline">
                    <ChevronLeft size={15} /> Назад
                </Link>
                <span className="px-1 text-sm text-dim">
                    {cats.length} {cats.length === 1 ? 'бланк' : cats.length < 5 ? 'бланки' : 'бланків'}
                </span>
                <Button size="sm" variant="primary" icon={Printer} onClick={() => window.print()}>
                    Друкувати
                </Button>
            </div>
            {cats.map((c) => (
                <Sheet key={c.id} header={header} number={c.number} category={c} />
            ))}
        </div>
    );
}
