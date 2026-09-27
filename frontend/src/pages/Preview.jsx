import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ChevronLeft, Play } from 'lucide-react';
import { useLive } from '../lib/live';
import RemoteControls from '../show/RemoteControls';
import Stage from '../show/Stage';
import { unlockAudio } from '../show/sound';
import { LiveDot } from '../ui/TopBar';

// Rehearsal: screen and remote side by side on one device.
export default function Preview() {
    const { pid } = useParams();
    const { show, offset, command, status } = useLive(pid, 'remote');
    const [unlocked, setUnlocked] = useState(false);

    return (
        <div className="mx-auto max-w-[1600px] px-4 py-4">
            <header className="mb-4 flex items-center justify-between gap-2">
                <div className="flex items-center gap-3">
                    <Link to={`/p/${pid}/show`} className="btn btn-ghost btn-sm text-dim no-underline">
                        <ChevronLeft size={16} /> Проєкт
                    </Link>
                    <div>
                        <div className="font-display font-bold">Репетиція</div>
                        <div className="text-xs text-dim">Той самий показ, що й на екрані — команди з пульта бачать усі пристрої</div>
                    </div>
                </div>
                <LiveDot status={status} />
            </header>
            <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_380px]">
                <div className="relative aspect-video overflow-hidden rounded-2xl border border-line bg-black shadow-2xl">
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
                <RemoteControls view={show} offset={offset} command={command} status={status} />
            </div>
        </div>
    );
}
