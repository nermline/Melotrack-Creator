// Small synthesised cues (no audio files needed). Browsers only allow sound
// after a user gesture, so the screen calls unlock() from its start overlay.
let ctx = null;

export function unlockAudio() {
    const AC = window.AudioContext || window.webkitAudioContext;
    if (!ctx && AC) ctx = new AC();
    if (ctx?.state === 'suspended') ctx.resume().catch(() => {});
}

function tone(freq, start, dur, { type = 'sine', gain = 0.2 } = {}) {
    if (!ctx || ctx.state !== 'running') return;
    const t = ctx.currentTime + start;
    const osc = ctx.createOscillator();
    const g = ctx.createGain();
    osc.type = type;
    osc.frequency.setValueAtTime(freq, t);
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(gain, t + 0.01);
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    osc.connect(g).connect(ctx.destination);
    osc.start(t);
    osc.stop(t + dur + 0.05);
}

export const cues = {
    tick: (urgent) => tone(urgent ? 1046 : 784, 0, urgent ? 0.09 : 0.06, { type: 'square', gain: urgent ? 0.12 : 0.07 }),
    go: () => {
        tone(523, 0, 0.18, { type: 'triangle', gain: 0.18 });
        tone(784, 0.08, 0.3, { type: 'triangle', gain: 0.18 });
    },
    reveal: () => [523, 659, 784].forEach((f, i) => tone(f, i * 0.07, 0.35, { type: 'triangle', gain: 0.14 })),
    fanfare: () =>
        [523, 659, 784, 1046, 784, 1046].forEach((f, i) =>
            tone(f, i * 0.12, i === 5 ? 0.9 : 0.2, { type: 'sawtooth', gain: 0.08 }),
        ),
};
