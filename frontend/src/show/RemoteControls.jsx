import { useEffect, useState } from 'react';
import {
    ChevronDown,
    FastForward,
    ListOrdered,
    Monitor,
    Pause,
    Play,
    Repeat,
    Rewind,
    RotateCcw,
    SkipBack,
    SkipForward,
    Trophy,
    TriangleAlert,
} from 'lucide-react';
import { remainingMs, useTick } from '../lib/live';
import { fmtTime } from '../lib/format';
import { cx } from '../lib/cx';
import { useFeedback } from '../ui/feedbackContext';

const PHASE_LABEL = {
    welcome: 'Вітання',
    category: 'Назва категорії',
    countdown: 'Відлік',
    playing: 'Звучить пісня',
    thinking: 'Час на відповідь',
    collect: 'Збір бланків',
    answers: 'Правильні відповіді',
    standings: 'Проміжний залік',
    scoring: 'Підрахунок балів',
    results: 'Оголошення переможців',
    finished: 'Фінал',
};

function primaryAction(v) {
    switch (v?.phase) {
        case undefined:
        case 'welcome':
            return { label: 'Почати показ', icon: Play };
        case 'collect':
            return { label: 'Показати відповіді', icon: SkipForward };
        case 'answers':
        case 'standings':
            return v.cat_index + 1 < v.cat_count
                ? { label: 'Наступна категорія', icon: SkipForward }
                : { label: 'До результатів', icon: Trophy };
        case 'scoring':
            return { label: 'Оголосити результати', icon: Trophy };
        case 'results':
            return { label: v.revealed >= (v.board?.length || 0) ? 'Завершити' : 'Відкрити наступне місце', icon: Trophy };
        case 'finished':
            return null;
        default:
            return { label: 'Далі', icon: SkipForward };
    }
}

function BigButton({ icon: Icon, children, onClick, disabled, className }) {
    return (
        <button
            type="button"
            onClick={onClick}
            disabled={disabled}
            className={cx(
                'flex h-16 items-center justify-center gap-2 rounded-2xl border border-line-strong bg-white/[0.06] text-sm font-bold transition-colors active:scale-[0.98] disabled:opacity-35',
                className,
            )}
        >
            {Icon && <Icon size={20} />}
            {children}
        </button>
    );
}

// RemoteControls is the operator's panel: current state, the answer the host
// may need, and every command. Keyboard: Space/→ next, ← back, P pause, R replay.
export default function RemoteControls({ view, offset, command, status }) {
    const [more, setMore] = useState(false);
    const { confirm } = useFeedback();
    const timed = !!view?.timer?.duration_ms;
    useTick(timed && !view?.timer?.paused, 200);
    const rem = view ? remainingMs(view.timer, offset) : 0;
    const frac = timed ? 1 - rem / view.timer.duration_ms : 0;
    const primary = primaryAction(view);
    const host = view?.host;
    const phase = view?.phase;
    const inCategory = ['category', 'countdown', 'playing', 'thinking', 'collect', 'answers', 'standings'].includes(phase);

    useEffect(() => {
        const onKey = (e) => {
            if (/INPUT|TEXTAREA|SELECT/.test(e.target.tagName)) return;
            if (e.key === ' ' || e.key === 'ArrowRight' || e.key === 'PageDown') {
                e.preventDefault();
                command(phase === 'welcome' ? 'start' : 'next');
            } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') command('back');
            else if (e.key === 'p' || e.key === 'з') command('toggle_pause');
            else if (e.key === 'r' || e.key === 'к') command('replay');
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [command, phase]);

    const reset = async () => {
        if (await confirm({ title: 'Скинути показ?', message: 'Показ повернеться на вітальний екран. Бали команд не зміняться.', confirmLabel: 'Скинути', danger: true }))
            command('reset');
    };
    const toResults = async () => {
        if (await confirm({ title: 'Перейти до результатів?', message: 'Показ перескочить на екран підрахунку балів.', confirmLabel: 'Перейти' }))
            command('results');
    };

    const screens = view?.devices?.screen || 0;

    return (
        <div className="flex flex-col gap-3">
            <div className="panel p-4">
                <div className="flex items-center justify-between gap-2 text-xs text-dim">
                    <span className="inline-flex items-center gap-1.5">
                        <Monitor size={14} className={screens ? 'text-ok' : 'text-bad'} />
                        {screens ? `екран підключено${screens > 1 ? ` (${screens})` : ''}` : 'екран не підключено'}
                    </span>
                    {status !== 'online' && <span className="text-warn">звʼязок: {status}</span>}
                </div>
                <div className="mt-2 font-display text-lg font-bold">{view ? PHASE_LABEL[phase] : '…'}</div>
                {view && inCategory && view.cat_count > 0 && (
                    <div className="mt-0.5 text-sm text-dim">
                        Категорія {view.cat_index + 1}/{view.cat_count}: <b className="text-ink">{view.category?.title}</b>
                        {['countdown', 'playing', 'thinking'].includes(phase) && ` · пісня ${view.item_index + 1}/${view.item_count}`}
                    </div>
                )}
                {phase === 'results' && (
                    <div className="mt-0.5 text-sm text-dim">
                        Відкрито {view.revealed} з {view.board?.length || 0} місць
                    </div>
                )}
                {host?.current && (
                    <div className="mt-3 rounded-xl bg-accent/10 px-3 py-2">
                        <div className="text-[11px] font-bold tracking-wider text-accent uppercase">Зараз</div>
                        <div className="font-semibold">{host.current}</div>
                        {host.next && <div className="mt-1 text-xs text-dim">далі: {host.next}</div>}
                    </div>
                )}
                {timed && (
                    <div className="mt-3">
                        <div className="h-2 overflow-hidden rounded-full bg-white/10">
                            <div className="h-full rounded-full bg-accent" style={{ width: `${frac * 100}%` }} />
                        </div>
                        <div className="mt-1 flex justify-between font-mono text-xs text-dim">
                            <span>{view.timer.paused ? 'пауза' : 'залишилось'}</span>
                            <span>{fmtTime(rem / 1000, true)}</span>
                        </div>
                    </div>
                )}
                {host?.warnings?.length > 0 && inCategory && (
                    <div className="mt-3 flex flex-col gap-1">
                        {host.warnings.map((w) => (
                            <div key={w} className="flex items-start gap-1.5 text-xs text-warn">
                                <TriangleAlert size={13} className="mt-px shrink-0" /> {w}
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {primary && (
                <button
                    type="button"
                    onClick={() => command(phase === 'welcome' || !view ? 'start' : 'next')}
                    className="flex h-20 items-center justify-center gap-3 rounded-2xl bg-accent font-display text-lg font-bold text-accent-ink shadow-[0_12px_40px_-12px_rgb(255_90_122_/_0.9)] active:scale-[0.98]"
                >
                    <primary.icon size={24} />
                    {primary.label}
                </button>
            )}
            {phase === 'finished' && (
                <BigButton icon={RotateCcw} onClick={reset}>
                    На вітальний екран
                </BigButton>
            )}

            <div className="grid grid-cols-3 gap-2">
                <BigButton icon={SkipBack} onClick={() => command('back')} disabled={!view || phase === 'welcome'}>
                    Назад
                </BigButton>
                <BigButton
                    icon={view?.timer?.paused ? Play : Pause}
                    onClick={() => command('toggle_pause')}
                    disabled={!timed}
                    className={view?.timer?.paused ? 'border-accent-2/60 text-accent-2' : ''}
                >
                    {view?.timer?.paused ? 'Продовжити' : 'Пауза'}
                </BigButton>
                <BigButton icon={Repeat} onClick={() => command('replay')} disabled={!['playing', 'thinking'].includes(phase)}>
                    Ще раз
                </BigButton>
                <BigButton icon={Rewind} onClick={() => command('seek', -5)} disabled={!timed}>
                    −5 с
                </BigButton>
                <BigButton icon={FastForward} onClick={() => command('seek', 5)} disabled={!timed}>
                    +5 с
                </BigButton>
                <BigButton icon={ListOrdered} onClick={() => command('standings')} disabled={!['answers', 'collect', 'category'].includes(phase) || !host?.teams}>
                    Залік
                </BigButton>
            </div>

            <div className="panel">
                <button type="button" onClick={() => setMore((m) => !m)} className="flex w-full items-center justify-between px-4 py-3 text-sm font-bold">
                    Перейти до…
                    <ChevronDown size={16} className={cx('transition-transform', more && 'rotate-180')} />
                </button>
                {more && (
                    <div className="border-t border-line p-2">
                        {(host?.categories || []).map((c, i) => (
                            <button
                                key={i}
                                type="button"
                                onClick={() => command('goto', i)}
                                className={cx(
                                    'flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm hover:bg-white/5',
                                    inCategory && view.cat_index === i && 'bg-accent/10',
                                )}
                            >
                                <span className="w-5 font-display font-bold text-faint">{i + 1}</span>
                                {c}
                            </button>
                        ))}
                        <div className="mt-2 grid grid-cols-2 gap-2 border-t border-line pt-2">
                            <button type="button" onClick={toResults} className="btn btn-sm">
                                <Trophy size={14} /> Результати
                            </button>
                            <button type="button" onClick={reset} className="btn btn-sm btn-danger">
                                <RotateCcw size={14} /> Скинути показ
                            </button>
                        </div>
                    </div>
                )}
            </div>

            {inCategory && host?.answers?.length > 0 && (
                <details className="panel px-4 py-3 text-sm">
                    <summary className="cursor-pointer font-bold">Відповіді категорії (лише для ведучого)</summary>
                    <ol className="mt-2 mb-0 pl-5 text-dim">
                        {host.answers.map((a, i) => (
                            <li key={i} className={cx('py-0.5', ['playing', 'thinking', 'countdown'].includes(phase) && i === view.item_index && 'font-bold text-ink')}>
                                {a || '—'}
                            </li>
                        ))}
                    </ol>
                </details>
            )}
            <p className="m-0 hidden text-center text-xs text-faint sm:block">
                Клавіші: Пробіл/→ — далі, ← — назад, P — пауза, R — ще раз
            </p>
        </div>
    );
}
