import { useEffect } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ChevronLeft } from 'lucide-react';
import { useLive } from '../lib/live';
import RemoteControls from '../show/RemoteControls';
import { LiveDot } from '../ui/TopBar';

// The operator's phone.
export default function Remote() {
    const { pid } = useParams();
    const { show, offset, command, status } = useLive(pid, 'remote');

    const title = show?.title;
    useEffect(() => {
        document.title = title ? `Пульт — ${title}` : 'Пульт';
    }, [title]);

    return (
        <div className="mx-auto min-h-screen max-w-lg px-3 pt-3 pb-8">
            <header className="mb-3 flex items-center justify-between gap-2">
                <Link to={`/p/${pid}/show`} className="btn btn-ghost btn-sm text-dim no-underline">
                    <ChevronLeft size={16} /> Проєкт
                </Link>
                <div className="min-w-0 truncate text-sm font-semibold">{show?.title}</div>
                <LiveDot status={status} />
            </header>
            <RemoteControls view={show} offset={offset} command={command} status={status} />
        </div>
    );
}
