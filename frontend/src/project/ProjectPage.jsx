import { NavLink, useNavigate, useParams } from 'react-router-dom';
import { ChevronLeft, ClipboardCheck, ListMusic, MonitorPlay, Users } from 'lucide-react';
import { api } from '../lib/api';
import { Button, InlineEdit, Spinner } from '../ui';
import { cx } from '../lib/cx';
import { useFeedback } from '../ui/feedbackContext';
import TopBar, { LiveDot } from '../ui/TopBar';
import { useProjectStore } from './useProjectStore';
import ProgramTab from './ProgramTab';
import TeamsTab from './TeamsTab';
import ScoringTab from './ScoringTab';
import ShowTab from './ShowTab';

const TABS = [
    { id: 'program', label: 'Програма', icon: ListMusic },
    { id: 'teams', label: 'Команди', icon: Users },
    { id: 'scoring', label: 'Оцінювання', icon: ClipboardCheck },
    { id: 'show', label: 'Показ', icon: MonitorPlay },
];

export default function ProjectPage() {
    const { pid, tab = 'program' } = useParams();
    const navigate = useNavigate();
    const store = useProjectStore(pid);
    const { project, error } = store;
    const { toast } = useFeedback();

    const rename = async (title) => {
        if (!title) return;
        try {
            const p = await api.patch(`/api/projects/${pid}`, { title });
            store.dispatch({ type: 'patch', patch: { title: p.title } });
        } catch (e) {
            toast(e.message, 'bad');
        }
    };

    return (
        <>
            <TopBar right={<LiveDot status={store.live.status} />}>
                <Button variant="ghost" size="sm" icon={ChevronLeft} onClick={() => navigate('/')} className="shrink-0">
                    <span className="hidden sm:inline">Мелотреки</span>
                </Button>
                {project && (
                    <InlineEdit
                        value={project.title}
                        onSave={rename}
                        maxLength={120}
                        className="font-semibold"
                        inputClassName="max-w-sm"
                    />
                )}
            </TopBar>

            <nav className="sticky top-14 z-20 border-b border-line bg-bg/80 backdrop-blur-md">
                <div className="mx-auto flex max-w-[1500px] gap-1 overflow-x-auto px-3">
                    {TABS.map((t) => (
                        <NavLink
                            key={t.id}
                            to={`/p/${pid}/${t.id}`}
                            className={() =>
                                cx(
                                    'relative inline-flex shrink-0 items-center gap-2 px-3 py-3 text-sm font-semibold no-underline transition-colors',
                                    tab === t.id ? 'text-ink' : 'text-dim hover:text-ink',
                                )
                            }
                        >
                            <t.icon size={16} />
                            {t.label}
                            {tab === t.id && <span className="absolute inset-x-2 bottom-0 h-0.5 rounded-full bg-accent" />}
                        </NavLink>
                    ))}
                </div>
            </nav>

            <main className="mx-auto max-w-[1500px] px-4 py-6">
                {error && <p className="text-bad">{error}</p>}
                {!project && !error && (
                    <div className="py-16 text-center text-dim">
                        <Spinner size={22} />
                    </div>
                )}
                {project && tab === 'program' && <ProgramTab store={store} />}
                {project && tab === 'teams' && <TeamsTab store={store} />}
                {project && tab === 'scoring' && <ScoringTab store={store} />}
                {project && tab === 'show' && <ShowTab store={store} />}
            </main>
        </>
    );
}
