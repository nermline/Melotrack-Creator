import { useEffect, useRef, useState } from 'react';
import { remainingMs, mediaUrl, answerImg } from './useGame';

// Стилі тримаємо інлайн і мінімальними (clamp → адаптивний розмір і в фуллскріні,
// і в маленькому превʼю комбо-режиму редактора).
const S = {
    wrap: { position: 'relative', width: '100%', height: '100%', background: '#000',
        color: '#fff', overflow: 'hidden', fontFamily: 'system-ui, sans-serif' },
    center: { position: 'absolute', inset: 0, display: 'flex', alignItems: 'center',
        justifyContent: 'center', textAlign: 'center', flexDirection: 'column', padding: '2%' },
    big: { fontSize: 'clamp(26px, 8vw, 120px)', fontWeight: 800, lineHeight: 1.05 },
    huge: { fontSize: 'clamp(60px, 20vw, 340px)', fontWeight: 900, lineHeight: 1 },
    sub: { fontSize: 'clamp(13px, 2.4vw, 34px)', color: '#aaa', marginTop: '1.4vh' },
    cats: { display: 'flex', flexWrap: 'wrap', gap: '1vw', justifyContent: 'center',
        maxWidth: '92%', marginTop: '4vh' },
    chip: { background: '#1c1c22', padding: '0.8vh 1.6vw', borderRadius: 8,
        fontSize: 'clamp(12px, 2vw, 28px)' },
    video: { maxWidth: '100%', maxHeight: '100%', background: '#000' },
    audio: { fontSize: 'clamp(40px, 12vw, 180px)' },
    answersTitle: { fontSize: 'clamp(18px, 4vw, 56px)', fontWeight: 800, textAlign: 'center',
        padding: '2vh 0 1vh' },
    grid: { display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '1.2vw',
        padding: '0 2vw 2vh', width: '100%', boxSizing: 'border-box' },
    cell: { background: '#14141a', borderRadius: 10, padding: '0.8vh' },
    cellImg: { width: '100%', aspectRatio: '1 / 1', objectFit: 'cover', borderRadius: 6,
        background: '#222', display: 'block' },
    cellTxt: { marginTop: '0.5vh', fontSize: 'clamp(11px, 1.5vw, 24px)', textAlign: 'center' },
    overlay: { position: 'absolute', inset: 0, background: '#000', zIndex: 5, cursor: 'pointer',
        display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column' },
};

export default function GameScreen({ pid, cats, state, offsetRef }) {
    const [, force] = useState(0);
    const [unlocked, setUnlocked] = useState(false);
    const videoRef = useRef(null);

    // Тік ~10/с для оновлення таймерів і синхронізації відео.
    useEffect(() => {
        const id = setInterval(() => force(n => (n + 1) % 1000000), 100);
        return () => clearInterval(id);
    }, []);

    // Синхронізація відео з серверним таймлайном (перемотка/пауза/доєднання).
    useEffect(() => {
        const v = videoRef.current;
        if (!v || !state) return;
        if (state.phase !== 'playing') { v.pause(); return; }
        const dur = state.phase_duration_ms || 0;
        const target = (dur - remainingMs(state, offsetRef.current)) / 1000;
        if (isFinite(target) && Math.abs(v.currentTime - target) > 0.75) {
            try { v.currentTime = target; } catch {}
        }
        if (state.paused) v.pause();
        else if (unlocked) v.play().catch(() => {});
    });

    if (!state) {
        return <div style={S.wrap}><div style={S.center}><div style={S.sub}>Підключення…</div></div></div>;
    }

    const cat = cats[state.category_index];
    const item = cat?.items?.[state.item_index];

    return (
        <div style={S.wrap}>
            {!unlocked && (
                <div style={S.overlay} onClick={() => setUnlocked(true)}>
                    <div style={S.big}>▶</div>
                    <div style={S.sub}>Натисніть, щоб увімкнути показ (дозвіл звуку)</div>
                </div>
            )}
            {renderPhase()}
        </div>
    );

    function renderPhase() {
        const sec = Math.ceil(remainingMs(state, offsetRef.current) / 1000);

        switch (state.phase) {
            case 'welcome':
                return (
                    <div style={S.center}>
                        <div style={S.big}>Ласкаво просимо!</div>
                        <div style={S.cats}>
                            {cats.map(c => <span key={c.id} style={S.chip}>{c.title}</span>)}
                        </div>
                    </div>
                );

            case 'category_title':
                return (
                    <div style={S.center}>
                        <div style={S.sub}>Категорія {state.category_index + 1} / {cats.length}</div>
                        <div style={S.big}>{cat?.title || ''}</div>
                    </div>
                );

            case 'countdown':
                return <div style={S.center}><div style={S.huge}>{sec}</div></div>;

            case 'playing':
                return (
                    <div style={S.center}>
                        {state.show_video && state.has_clip && cat && item ? (
                            <video
                                key={`${cat.id}-${item.id}`}
                                ref={videoRef}
                                style={S.video}
                                playsInline
                                src={mediaUrl(pid, cat.id, item.id)}
                            />
                        ) : (
                            <>
                                {/* Лише аудіо — відео сховане, але елемент потрібен для звуку */}
                                {cat && item && state.has_clip && (
                                    <video
                                        key={`${cat.id}-${item.id}`}
                                        ref={videoRef}
                                        style={{ display: 'none' }}
                                        playsInline
                                        src={mediaUrl(pid, cat.id, item.id)}
                                    />
                                )}
                                <div style={S.audio}>🎵</div>
                                <div style={S.sub}>Звучить мелодія…</div>
                            </>
                        )}
                    </div>
                );

            case 'thinking':
                return (
                    <div style={S.center}>
                        <div style={S.sub}>Час на відповідь</div>
                        <div style={S.huge}>{sec}</div>
                    </div>
                );

            case 'await_answers':
                return (
                    <div style={S.center}>
                        <div style={S.big}>Час вийшов!</div>
                        <div style={S.sub}>Готуємось показати відповіді…</div>
                    </div>
                );

            case 'answers':
                return (
                    <div style={{ position: 'absolute', inset: 0, display: 'flex',
                        flexDirection: 'column', justifyContent: 'center' }}>
                        <div style={S.answersTitle}>{cat?.title || 'Відповіді'}</div>
                        <div style={S.grid}>
                            {(cat?.items || []).map(it => (
                                <div key={it.id} style={S.cell}>
                                    <img
                                        style={{ ...S.cellImg, visibility: it.answer?.image_path ? 'visible' : 'hidden' }}
                                        src={it.answer?.image_path ? answerImg(it.answer.image_path) : undefined}
                                        onError={e => { e.target.style.visibility = 'hidden'; }}
                                        alt=""
                                    />
                                    <div style={S.cellTxt}>{it.answer?.title || ''}</div>
                                </div>
                            ))}
                        </div>
                    </div>
                );

            case 'finished':
                return <div style={S.center}><div style={S.big}>Дякуємо за гру! 🎉</div></div>;

            default:
                return <div style={S.center}><div style={S.big}>{state.phase}</div></div>;
        }
    }
}
