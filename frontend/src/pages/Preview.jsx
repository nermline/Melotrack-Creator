import { useEffect, useRef, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { ChevronLeft, Play } from 'lucide-react';
import { useLive } from '../lib/live';
import RemoteControls from '../show/RemoteControls';
import Stage from '../show/Stage';
import { unlockAudio } from '../show/sound';
import { LiveDot } from '../ui/TopBar';

// rehearsalId names this tab's private rehearsal. It survives a reload of the
// tab, while another tab or another organiser gets a rehearsal of their own.
function rehearsalId(pid) {
    const key = `melotrack:rehearsal:${pid}`;
    try {
        const saved = sessionStorage.getItem(key);
        if (saved) return saved;
    } catch {
        /* storage unavailable: a fresh rehearsal per load */
    }
    const id = `r-${crypto.getRandomValues(new Uint32Array(2)).reduce((s, n) => s + n.toString(36), '')}`.slice(0, 24);
    try {
        sessionStorage.setItem(key, id);
    } catch {
        /* ignore */
    }
    return id;
}

// Rehearsal: screen and remote side by side on one device, in a private copy
// of the show. ?cat=N opens category N straight away.
export default function Preview() {
    const { pid } = useParams();
    const [params, setParams] = useSearchParams();
    const [session] = useState(() => rehearsalId(pid));
    const { show, offset, command, status } = useLive(pid, 'remote', undefined, session);
    const [unlocked, setUnlocked] = useState(false);

    // Jump to the requested category once the rehearsal is connected.
    const cat = params.get('cat');
    const jumped = useRef(false);
    useEffect(() => {
        if (cat == null || jumped.current || status !== 'online' || !show) return;
        const i = Number(cat);
        if (Number.isInteger(i) && i >= 0 && i < show.cat_count && command('goto', i)) {
            jumped.current = true;
            setParams({}, { replace: true });
        }
    }, [cat, status, show, command, setParams]);

    const title = show?.title;
    useEffect(() => {
        document.title = title ? `Репетиція — ${title}` : 'Репетиція';
    }, [title]);

    return (
        <div className="mx-auto max-w-[1600px] px-4 py-4">
            <header className="mb-4 flex items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-3">
                    <Link to={`/p/${pid}/program`} className="btn btn-ghost btn-sm text-dim no-underline">
                        <ChevronLeft size={16} /> Проєкт
                    </Link>
                    <div className="min-w-0">
                        <div className="font-display font-bold">Репетиція</div>
                        <div className="truncate text-xs text-dim">Так виглядатиме екран. Справжній показ не зміниться</div>
                    </div>
                </div>
                <LiveDot status={status} />
            </header>
            <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_380px]">
                <div className="relative aspect-video overflow-hidden rounded-2xl border border-line bg-black shadow-2xl lg:sticky lg:top-4">
                    <Stage view={show} offset={offset} unlocked={unlocked} />
                    {!unlocked && (
                        <button
                            type="button"
                            onClick={() => {
                                unlockAudio();
                                setUnlocked(true);
                            }}
                            className="absolute inset-0 z-40 flex flex-col items-center justify-center gap-3 bg-black/70 text-ink backdrop-blur-sm"
                        >
                            <span className="grid h-16 w-16 place-items-center rounded-full bg-accent text-accent-ink">
                                <Play size={28} fill="currentColor" />
                            </span>
                            <span className="font-semibold">Увімкнути звук</span>
                        </button>
                    )}
                </div>
                <RemoteControls view={show} offset={offset} command={command} status={status} rehearsal />
            </div>
        </div>
    );
}
