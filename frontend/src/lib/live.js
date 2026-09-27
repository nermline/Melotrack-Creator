import { useCallback, useEffect, useRef, useState } from 'react';

const wsBase = () => `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}`;

// useLive keeps one WebSocket per project open, reconnecting with backoff.
// `show` is the latest show state; other events go to onEvent.
// `offset` converts local time to server time (serverNow ≈ Date.now() + offset).
export function useLive(pid, role, onEvent) {
    const [status, setStatus] = useState('connecting');
    const [show, setShow] = useState(null);
    const [offset, setOffset] = useState(0);
    const wsRef = useRef(null);
    const onEventRef = useRef(onEvent);
    useEffect(() => {
        onEventRef.current = onEvent;
    }, [onEvent]);

    useEffect(() => {
        if (!pid) return;
        let alive = true;
        let retry = 0;
        let timer = null;
        let lastSeq = -1;

        const connect = () => {
            if (!alive) return;
            setStatus(retry === 0 ? 'connecting' : 'reconnecting');
            const ws = new WebSocket(`${wsBase()}/api/projects/${pid}/live?role=${role}`);
            wsRef.current = ws;
            ws.onopen = () => {
                if (!alive) return;
                retry = 0;
                lastSeq = -1;
                setStatus('online');
                // A reconnect may have missed events: let the page resync.
                onEventRef.current?.({ type: 'resync' });
            };
            ws.onmessage = (e) => {
                let msg;
                try {
                    msg = JSON.parse(e.data);
                } catch {
                    return;
                }
                if (msg.type === 'show') {
                    const v = msg.data;
                    if (v.seq < lastSeq) return;
                    lastSeq = v.seq;
                    setOffset(v.server_now - Date.now());
                    setShow(v);
                } else {
                    onEventRef.current?.(msg);
                }
            };
            ws.onclose = (e) => {
                if (!alive) return;
                if (e.code === 1008 || e.code === 4401) {
                    setStatus('offline');
                    return;
                }
                setStatus('reconnecting');
                retry++;
                timer = setTimeout(connect, Math.min(1000 * 2 ** Math.min(retry, 4), 10000));
            };
            ws.onerror = () => {};
        };
        connect();

        // Phones kill sockets in the background; reconnect as soon as the page is visible.
        const onVisible = () => {
            if (document.visibilityState === 'visible' && wsRef.current?.readyState === WebSocket.CLOSED) {
                clearTimeout(timer);
                retry = 0;
                connect();
            }
        };
        document.addEventListener('visibilitychange', onVisible);
        return () => {
            alive = false;
            clearTimeout(timer);
            document.removeEventListener('visibilitychange', onVisible);
            wsRef.current?.close();
        };
    }, [pid, role]);

    const command = useCallback((action, value = 0) => {
        const ws = wsRef.current;
        if (ws?.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'cmd', action, value }));
            return true;
        }
        return false;
    }, []);

    return { status, show, offset, command };
}

// Remaining time of the current phase in ms, from the server clock.
export function remainingMs(timer, offset) {
    if (!timer || !timer.duration_ms) return 0;
    if (timer.paused) return timer.remaining_ms || 0;
    return Math.max(0, timer.ends_at - (Date.now() + offset));
}

// useTick re-renders the component on every animation frame (or every `ms`) while active.
export function useTick(active, ms = 0) {
    const [, setN] = useState(0);
    useEffect(() => {
        if (!active) return;
        let id;
        if (ms > 0) {
            id = setInterval(() => setN((n) => (n + 1) % 1e9), ms);
            return () => clearInterval(id);
        }
        const loop = () => {
            setN((n) => (n + 1) % 1e9);
            id = requestAnimationFrame(loop);
        };
        id = requestAnimationFrame(loop);
        return () => cancelAnimationFrame(id);
    }, [active, ms]);
}
