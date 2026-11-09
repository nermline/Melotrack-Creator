// The show's sound: synthesised cues (no audio files needed) and the clips,
// both through one master volume set from the remote. Browsers only allow
// sound after a user gesture, so the screen calls unlockAudio() from its
// start overlay.
let ctx = null;
let master = null;
let volume = 1;

export function unlockAudio() {
    const AC = window.AudioContext || window.webkitAudioContext;
    if (!ctx && AC) ctx = new AC();
    if (ctx?.state === 'suspended') ctx.resume().catch(() => {});
}

// output is the master volume, followed by a limiter so that turning the
// show up past 100% gets louder without clipping.
function output() {
    if (!ctx) return null;
    if (!master) {
        master = ctx.createGain();
        master.gain.value = volume;
        const limiter = ctx.createDynamicsCompressor();
        limiter.threshold.value = -1;
        limiter.knee.value = 0;
        limiter.ratio.value = 20;
        limiter.attack.value = 0.003;
        limiter.release.value = 0.25;
        master.connect(limiter).connect(ctx.destination);
    }
    return master;
}

// setVolume sets the show's volume (1 = as cut, up to 2).
export function setVolume(v) {
    volume = v;
    if (master) master.gain.setTargetAtTime(v, ctx.currentTime, 0.05);
}

const routed = new WeakSet();

// routeMedia sends a clip's sound through the show's volume. Until sound is
// unlocked the element plays on its own, at most at full volume.
export function routeMedia(el) {
    if (!el || routed.has(el)) return;
    const out = output();
    if (!out) {
        el.volume = Math.min(volume, 1);
        return;
    }
    try {
        ctx.createMediaElementSource(el).connect(out);
        el.volume = 1;
        routed.add(el);
    } catch {
        el.volume = Math.min(volume, 1); // WebAudio refused: the element's own volume still works
    }
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
    osc.connect(g).connect(output());
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
