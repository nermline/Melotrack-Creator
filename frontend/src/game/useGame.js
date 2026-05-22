import { useEffect, useRef, useState, useCallback } from 'react';
import api from '../api';

const WS_BASE = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}`;

// Людські назви фаз для пульта/статусу.
export const PHASE_LABELS = {
    welcome: 'Привітання',
    category_title: 'Назва категорії',
    countdown: 'Відлік',
    playing: 'Відтворення',
    thinking: 'Роздуми',
    await_answers: 'Очікування відповідей',
    answers: 'Відповіді',
    finished: 'Кінець',
};

export function token() {
    return localStorage.getItem('token') || '';
}

export function mediaUrl(pid, catId, itemId) {
    return `/media/${pid}/${catId}/${itemId}.mp4?token=${encodeURIComponent(token())}`;
}

export function answerImg(path) {
    return `${path}?token=${encodeURIComponent(token())}`;
}

// Час, що лишився у поточній фазі (мс), з урахуванням зсуву годинника сервер↔клієнт.
export function remainingMs(state, offset) {
    if (!state) return 0;
    if (state.paused) return state.remaining_ms || 0;
    if (!state.phase_ends_at) return 0;
    return Math.max(0, state.phase_ends_at - (Date.now() + offset));
}

// useProject завантажує проєкт один раз і повертає відсортовані категорії/питання.
export function useProject(pid) {
    const [project, setProject] = useState(null);
    const [cats, setCats] = useState([]);
    const [error, setError] = useState('');

    useEffect(() => {
        let alive = true;
        api.get(`/api/projects/${pid}`)
            .then(r => {
                if (!alive) return;
                setProject(r.data);
                const sorted = [...(r.data.categories || [])]
                    .sort((a, b) => a.position - b.position)
                    .map(c => ({
                        ...c,
                        items: [...(c.items || [])].sort((a, b) => a.position - b.position),
                    }));
                setCats(sorted);
            })
            .catch(() => alive && setError('Не вдалося завантажити проєкт'));
        return () => { alive = false; };
    }, [pid]);

    return { project, cats, error };
}

// useGameWS відкриває WebSocket ігрової сесії з заданою роллю (screen|remote).
// Повертає поточний стан, статус зʼєднання, зсув годинника та функцію надсилання команд.
export function useGameWS(pid, role) {
    const [state, setState] = useState(null);
    const [status, setStatus] = useState('connecting');
    const offsetRef = useRef(0);
    const wsRef = useRef(null);

    useEffect(() => {
        if (!pid) return;
        let alive = true;
        let timer = null;

        function connect() {
            if (!alive) return;
            const ws = new WebSocket(
                `${WS_BASE}/api/projects/${pid}/ws?role=${role}&token=${encodeURIComponent(token())}`
            );
            wsRef.current = ws;
            setStatus('connecting');
            ws.onopen = () => { if (alive) setStatus('connected'); };
            ws.onerror = () => {};
            ws.onclose = () => {
                if (!alive) return;
                setStatus('disconnected');
                timer = setTimeout(connect, 2000);
            };
            ws.onmessage = (e) => {
                try {
                    const msg = JSON.parse(e.data);
                    if (msg.event === 'state_updated') {
                        offsetRef.current = (msg.state.server_now || Date.now()) - Date.now();
                        setState(msg.state);
                    }
                } catch { /* ignore malformed frame */ }
            };
        }

        connect();
        return () => {
            alive = false;
            clearTimeout(timer);
            wsRef.current?.close();
        };
    }, [pid, role]);

    const send = useCallback((action, value = 0) => {
        const ws = wsRef.current;
        if (ws?.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ action, value }));
        }
    }, []);

    return { state, status, offsetRef, send };
}
