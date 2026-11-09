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

// gainOf turns the remote's volume into a gain on a perceptual scale: every
// doubling is +10 dB (about twice as loud), so 200% sounds twice as loud as
// 100%, 400% is +20 dB and 50% half as loud.
function gainOf(v) {
    return v <= 0 ? 0 : 10 ** ((10 * Math.log2(v)) / 20);
}

// softClip leaves the signal alone below 0.9 of full scale and rounds off
// anything above, so a big boost saturates instead of cracking.
function softClip() {
    const n = 2048;
    const curve = new Float32Array(n);
    for (let i = 0; i < n; i++) {
        const x = (i / (n - 1)) * 2 - 1;
        const a = Math.abs(x);
        curve[i] = a <= 0.9 ? x : Math.sign(x) * (0.9 + 0.1 * Math.tanh((a - 0.9) / 0.1));
    }
    const shaper = ctx.createWaveShaper();
    shaper.curve = curve;
    shaper.oversample = '4x';
    return shaper;
}

// output is the master volume, then a fast limiter and a soft clipper: at
// 100% the clips (normalised, peaks under -1.5 dB) pass untouched, and
// turning the show up gets much louder without clipping.
function output() {
    if (!ctx) return null;
    if (!master) {
        master = ctx.createGain();
        master.gain.value = gainOf(volume);
        const limiter = ctx.createDynamicsCompressor();
        limiter.threshold.value = -1;
        limiter.knee.value = 0;
        limiter.ratio.value = 20;
        limiter.attack.value = 0.001;
        limiter.release.value = 0.12;
        master.connect(limiter).connect(softClip()).connect(ctx.destination);
    }
    return master;
}

// setVolume sets the show's volume as the remote shows it (1 = 100%, as cut).
export function setVolume(v) {
    volume = v;
    if (master) master.gain.setTargetAtTime(gainOf(v), ctx.currentTime, 0.05);
}

const routed = new WeakSet();

// routeMedia sends a clip's sound through the show's volume. Until sound is
// unlocked the element plays on its own, at most at full volume.
export function routeMedia(el) {
    if (!el || routed.has(el)) return;
    const out = output();
    if (!out) {
        el.volume = Math.min(gainOf(volume), 1);
        return;
    }
    try {
        ctx.createMediaElementSource(el).connect(out);
        el.volume = 1;
        routed.add(el);
    } catch {
        el.volume = Math.min(gainOf(volume), 1); // WebAudio refused: the element's own volume still works
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
