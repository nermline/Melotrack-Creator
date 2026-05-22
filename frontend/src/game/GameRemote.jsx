import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { remainingMs, PHASE_LABELS } from './useGame';
import { Button } from '../ui/kit';

function fmt(ms) {
    const s = Math.max(0, Math.ceil(ms / 1000));
    return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

export default function GameRemote({ state, offsetRef, send, cats = [] }) {
    const [, force] = useState(0);

    // Тік для прогрес-бару поточної фази.
    useEffect(() => {
        const id = setInterval(() => force((n) => (n + 1) % 1000000), 100);
        return () => clearInterval(id);
    }, []);

    const phase = state?.phase;
    const hasTimer = (state?.phase_duration_ms || 0) > 0;
    const rem = state ? remainingMs(state, offsetRef.current) : 0;
    const pct = hasTimer ? Math.max(0, Math.min(100, 100 * (1 - rem / state.phase_duration_ms))) : 0;
    const paused = !!state?.paused;

    // Заголовки беремо з завантаженого проєкту (cats) за індексами стану.
    const inGame = state && phase !== 'welcome' && phase !== 'finished';
    const cat = inGame ? cats[state.category_index] : null;
    const item = cat?.items?.[state.item_index];
    const itemActive = phase === 'countdown' || phase === 'playing' || phase === 'thinking';
    const itemTitle = itemActive ? (item?.answer?.title || 'без назви') : null;

    const meta = [];
    if (state?.total_categories) meta.push(`Категорія ${state.category_index + 1}/${state.total_categories}`);
    if (inGame && state.total_items) meta.push(`Питання ${state.item_index + 1}/${state.total_items}`);
    if (phase === 'playing') { meta.push(state.show_video ? '📺 відео' : '🎵 аудіо'); if (!state.has_clip) meta.push('⚠️ нема кліпу'); }

    // Контекстна головна дія.
    const primary = (() => {
        if (!state || phase === 'welcome') return { label: '▶ Почати показ', on: () => send('start') };
        if (phase === 'finished') return { label: '↻ Почати спочатку', on: () => send('start') };
        if (phase === 'await_answers') return { label: '✅ Показати відповіді', on: () => send('next') };
        if (phase === 'answers') return { label: '⏭ Наступна категорія', on: () => send('next') };
        return { label: '⏭ Далі', on: () => send('next') };
    })();

    const ctrlStyle = { padding: 14, fontSize: 15 };

    return (
        <div className="grid gap-4" style={{ maxWidth: 540 }}>
            {/* Статус */}
            <div className="glass p-4">
                <div className="flex items-center justify-between gap-2">
                    <span style={{ fontSize: 22, fontWeight: 800, color: '#fff' }}>
                        {state ? (PHASE_LABELS[phase] || phase) : '—'}
                    </span>
                    {paused && <span style={{ color: 'var(--color-warn)', fontWeight: 600 }}>⏸ пауза</span>}
                </div>
                {cat && (
                    <div className="mt-1 truncate" style={{ fontWeight: 600, color: '#fff' }} title={cat.title}>
                        📂 {cat.title}
                    </div>
                )}
                {itemTitle && (
                    <div className="truncate" style={{ color: 'var(--color-accent2)' }} title={itemTitle}>
                        🎵 {itemTitle}
                    </div>
                )}
                <div className="mt-1 text-sm" style={{ color: 'var(--color-muted)' }}>{meta.join(' · ') || '—'}</div>
                <div className="mt-3" style={{ position: 'relative', height: 10, borderRadius: 6,
                    background: 'rgba(255,255,255,0.08)', overflow: 'hidden' }}>
                    <motion.div animate={{ width: `${pct}%` }} transition={{ ease: 'linear', duration: 0.12 }}
                        style={{ position: 'absolute', top: 0, bottom: 0, left: 0,
                            background: 'linear-gradient(90deg, var(--color-accent2), var(--color-accent))' }} />
                </div>
                <div className="mt-1 flex justify-between text-xs" style={{ color: 'var(--color-muted)' }}>
                    <span>{hasTimer ? (paused ? 'на паузі' : 'залишилось') : 'без таймера'}</span>
                    <span style={{ fontFamily: 'var(--font-mono)' }}>{hasTimer ? fmt(rem) : '—'}</span>
                </div>
            </div>

            {/* Головна дія */}
            <button onClick={primary.on} className="btn btn-primary"
                style={{ fontSize: 18, padding: '18px 16px', borderRadius: 14 }}>
                {primary.label}
            </button>

            {/* Керування */}
            <div className="grid gap-2" style={{ gridTemplateColumns: '1fr 1fr' }}>
                <Button onClick={() => send('back')} disabled={!state || phase === 'welcome'} style={ctrlStyle}>⏮ Назад</Button>
                <Button onClick={() => send(paused ? 'resume' : 'pause')} disabled={!hasTimer} style={ctrlStyle}>
                    {paused ? '▶ Продовжити' : '⏸ Пауза'}
                </Button>
                <Button onClick={() => send('seek', -5)} disabled={!hasTimer} style={ctrlStyle}>⏪ −5 c</Button>
                <Button onClick={() => send('seek', 5)} disabled={!hasTimer} style={ctrlStyle}>⏩ +5 c</Button>
            </div>

            <Button variant="danger" onClick={() => { if (confirm('Скинути показ на привітання?')) send('reset'); }}
                style={ctrlStyle}>⟲ Скинути показ</Button>
        </div>
    );
}
