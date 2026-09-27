import { useSyncExternalStore } from 'react';

// One shared audio element for listening to clips in the editor, so starting a
// preview always stops the previous one.
let audio = null;
let current = null; // { id, url }
let progress = 0;
const listeners = new Set();
const emit = () => listeners.forEach((l) => l());

function el() {
    if (!audio) {
        audio = new Audio();
        audio.preload = 'auto';
        audio.addEventListener('ended', () => {
            current = null;
            emit();
        });
        audio.addEventListener('timeupdate', () => {
            progress = audio.duration ? audio.currentTime / audio.duration : 0;
            emit();
        });
    }
    return audio;
}

export function togglePreview(id, url) {
    const a = el();
    if (current?.id === id) {
        a.pause();
        current = null;
    } else {
        a.src = url;
        a.currentTime = 0;
        a.play().catch(() => {});
        current = { id, url };
        progress = 0;
    }
    emit();
}

export function stopPreview() {
    if (audio) audio.pause();
    current = null;
    emit();
}

const subscribe = (l) => {
    listeners.add(l);
    return () => listeners.delete(l);
};

export function usePreview(id) {
    const playing = useSyncExternalStore(subscribe, () => current?.id === id);
    const p = useSyncExternalStore(subscribe, () => (current?.id === id ? progress : 0));
    return { playing, progress: p };
}
