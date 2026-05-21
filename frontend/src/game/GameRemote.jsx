import { useEffect, useState } from 'react';
import { remainingMs, PHASE_LABELS } from './useGame';

const S = {
    box: { background: '#1f1f29', borderRadius: 8, padding: 12, marginBottom: 16,
        color: '#eee', fontFamily: 'system-ui, sans-serif' },
    phase: { fontSize: 20, fontWeight: 700, color: '#6ad' },
    meta: { fontSize: 14, lineHeight: 1.7, marginTop: 4 },
    bar: { height: 6, background: '#333', borderRadius: 3, overflow: 'hidden', marginTop: 8 },
    grid: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, maxWidth: 480 },
    btn: { fontSize: 16, padding: 14, border: 'none', borderRadius: 8,
        background: '#2d2d3a', color: '#fff', cursor: 'pointer' },
    primary: { background: '#2563eb' },
    warn: { background: '#b91c1c' },
    full: { gridColumn: '1 / -1' },
};

export default function GameRemote({ state, offsetRef, send }) {
    const [, force] = useState(0);

    // Тік для прогрес-бару поточної фази.
    useEffect(() => {
        const id = setInterval(() => force(n => (n + 1) % 1000000), 100);
        return () => clearInterval(id);
    }, []);

    const pct = state?.phase_duration_ms
        ? Math.max(0, Math.min(100, 100 * (1 - remainingMs(state, offsetRef.current) / state.phase_duration_ms)))
        : 0;

    const meta = [];
    if (state?.total_categories)
        meta.push(`Категорія ${state.category_index + 1}/${state.total_categories}`);
    if (state && state.phase !== 'welcome' && state.total_items)
        meta.push(`Питання ${state.item_index + 1}/${state.total_items}`);
    if (state?.phase === 'playing')
        meta.push(state.show_video ? '📺 відео' : '🎵 аудіо', state.has_clip ? '' : '⚠️ нема кліпу');
    if (state?.paused) meta.push('⏸ пауза');

    const btn = (extra) => ({ ...S.btn, ...extra });

    return (
        <div>
            <div style={S.box}>
                <div style={S.phase}>{state ? (PHASE_LABELS[state.phase] || state.phase) : '—'}</div>
                <div style={S.meta}>{meta.filter(Boolean).join(' · ')}</div>
                <div style={S.bar}><div style={{ height: '100%', background: '#2563eb', width: `${pct}%` }} /></div>
            </div>

            <div style={S.grid}>
                <button style={btn({ ...S.primary, ...S.full })} onClick={() => send('start')}>
                    ▶ Почати показ
                </button>

                <button style={btn()} onClick={() => send('back')}>⏮ Назад</button>
                <button style={btn(S.primary)} onClick={() => send('next')}>⏭ Далі / Пропустити</button>

                <button style={btn()} onClick={() => send(state?.paused ? 'resume' : 'pause')}>
                    {state?.paused ? '▶ Продовжити' : '⏸ Пауза'}
                </button>
                <button style={btn()} onClick={() => send('next')}>✅ Показати відповіді</button>

                <button style={btn()} onClick={() => send('seek', -5)}>⏪ −5 c</button>
                <button style={btn()} onClick={() => send('seek', 5)}>⏩ +5 c</button>

                <button style={btn({ ...S.warn, ...S.full })} onClick={() => send('reset')}>
                    ⟲ Скинути на привітання
                </button>
            </div>
        </div>
    );
}
