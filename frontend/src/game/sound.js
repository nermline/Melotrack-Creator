import { useCallback, useRef } from 'react';

// Синтез коротких «тіків» через Web Audio API — без зовнішніх аудіофайлів.
// AudioContext створюється лише після жесту користувача (клік «увімкнути показ»),
// інакше браузер блокує звук автоплеєм.
export function useTicker() {
    const ctxRef = useRef(null);

    // Створити/розбудити аудіоконтекст (викликати з обробника кліку).
    const unlock = useCallback(() => {
        if (!ctxRef.current) {
            const AC = window.AudioContext || window.webkitAudioContext;
            if (AC) ctxRef.current = new AC();
        }
        const ctx = ctxRef.current;
        if (ctx && ctx.state === 'suspended') ctx.resume().catch(() => {});
        return ctx;
    }, []);

    // Короткий клік: коротка обвідна гучності на осциляторі.
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
