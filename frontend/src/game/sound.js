import { useCallback, useRef } from 'react';

export function useTicker() {
    const ctxRef = useRef(null);

    const unlock = useCallback(() => {
        if (!ctxRef.current) {
            const AC = window.AudioContext || window.webkitAudioContext;
            if (AC) ctxRef.current = new AC();
        }
        const ctx = ctxRef.current;
        if (ctx && ctx.state === 'suspended') ctx.resume().catch(() => {});
        return ctx;
    }, []);

    const tick = useCallback(({ freq = 800, dur = 0.05, gain = 0.22, type = 'square' } = {}) => {
        const ctx = ctxRef.current;
        if (!ctx || ctx.state !== 'running') return;
        const t = ctx.currentTime;
        const osc = ctx.createOscillator();
        const g = ctx.createGain();
        osc.type = type;
        osc.frequency.setValueAtTime(freq, t);
        g.gain.setValueAtTime(0.0001, t);
        g.gain.exponentialRampToValueAtTime(gain, t + 0.004);
        g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
        osc.connect(g).connect(ctx.destination);
        osc.start(t);
        osc.stop(t + dur + 0.02);
    }, []);

    return { unlock, tick };
}
