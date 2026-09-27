import { useCallback, useEffect, useReducer, useRef } from 'react';
import { api } from '../lib/api';
import { useLive } from '../lib/live';

// The project page keeps the whole project (categories, songs, media, teams,
// scores) in one reducer. REST calls change it; live events from other
// organisers and from background jobs (downloads, renders) keep it current.

const initial = { project: null, error: '', scores: null, mediaProgress: {}, clipProgress: {} };

function mapItems(project, fn) {
    return {
        ...project,
        categories: project.categories.map((c) => ({ ...c, items: c.items.map(fn) })),
    };
}

function reducer(state, a) {
    switch (a.type) {
        case 'loaded':
            return { ...state, project: a.project, error: '' };
        case 'error':
            return { ...state, error: a.error };
        case 'patch':
            return state.project ? { ...state, project: { ...state.project, ...a.patch } } : state;
        case 'item': {
            if (!state.project) return state;
            const it = a.item;
            let found = false;
            const project = mapItems(state.project, (x) => {
                if (x.id !== it.id) return x;
                found = true;
                return it;
            });
            const clipProgress = { ...state.clipProgress };
            if (it.clip.status !== 'rendering') delete clipProgress[it.id];
            return found ? { ...state, project, clipProgress } : state;
        }
        case 'media': {
            if (!state.project) return state;
            const mediaProgress = { ...state.mediaProgress };
            if (a.media.status !== 'downloading') delete mediaProgress[a.media.id];
            return {
                ...state,
                mediaProgress,
                project: { ...state.project, media: { ...state.project.media, [a.media.id]: a.media } },
            };
        }
        case 'media_progress':
            return { ...state, mediaProgress: { ...state.mediaProgress, [a.id]: a.progress } };
        case 'item_progress':
            return { ...state, clipProgress: { ...state.clipProgress, [a.id]: a.progress } };
        case 'teams':
            return state.project ? { ...state, project: { ...state.project, teams: a.teams } } : state;
        case 'scores':
            return { ...state, scores: a.scores };
        case 'score': {
            const scores = { ...(state.scores || {}) };
            scores[`${a.team_id}:${a.item_id}`] = a.points;
            return { ...state, scores };
        }
        default:
            return state;
    }
}

export function scoreMap(list) {
    const m = {};
    for (const s of list) m[`${s.team_id}:${s.item_id}`] = s.points;
    return m;
}

export function useProjectStore(pid) {
    const [state, dispatch] = useReducer(reducer, initial);
    const reloadTimer = useRef(null);

    const reload = useCallback(async () => {
        try {
            const project = await api.get(`/api/projects/${pid}`);
            dispatch({ type: 'loaded', project });
        } catch (e) {
            dispatch({ type: 'error', error: e.status === 404 ? 'Мелотрек не знайдено' : e.message });
        }
    }, [pid]);

    const reloadScores = useCallback(async () => {
        try {
            dispatch({ type: 'scores', scores: scoreMap(await api.get(`/api/projects/${pid}/scores`)) });
        } catch {
            /* the scoring tab shows its own error */
        }
    }, [pid]);

    const scheduleReload = useCallback(() => {
        clearTimeout(reloadTimer.current);
        reloadTimer.current = setTimeout(reload, 120);
    }, [reload]);

    useEffect(() => {
        reload();
        reloadScores();
        return () => clearTimeout(reloadTimer.current);
    }, [reload, reloadScores]);

    const onEvent = useCallback(
        (msg) => {
            switch (msg.type) {
                case 'resync':
                    scheduleReload();
                    reloadScores();
                    break;
                case 'project':
                    scheduleReload();
                    break;
                case 'item':
                    dispatch({ type: 'item', item: msg.data });
                    break;
                case 'media':
                    dispatch({ type: 'media', media: msg.data });
                    break;
                case 'media_progress':
                    dispatch({ type: 'media_progress', ...msg.data });
                    break;
                case 'item_progress':
                    dispatch({ type: 'item_progress', ...msg.data });
                    break;
                case 'teams':
                    dispatch({ type: 'teams', teams: msg.data });
                    break;
                case 'score':
                    dispatch({ type: 'score', ...msg.data });
                    break;
                case 'scores_reload':
                    reloadScores();
                    break;
            }
        },
        [scheduleReload, reloadScores],
    );

    const live = useLive(pid, 'editor', onEvent);

    return { ...state, dispatch, reload, reloadScores, live };
}

// Helpers shared by the tabs.

export function allItems(project) {
    return project ? project.categories.flatMap((c) => c.items) : [];
}

// songState summarises what an organiser needs to know about a song.
export function songState(item, media) {
    if (!media) return { key: 'unknown', tone: 'muted', label: '—' };
    if (media.status === 'error') return { key: 'download_error', tone: 'bad', label: 'Помилка завантаження' };
    if (media.status === 'pending') return { key: 'queued', tone: 'muted', label: 'У черзі' };
    if (media.status === 'downloading')
        return { key: 'downloading', tone: 'info', label: media.kind === 'upload' ? 'Обробка файлу' : 'Завантаження' };
    switch (item.clip.status) {
        case 'ready':
            return { key: 'ready', tone: 'ok', label: 'Готово' };
        case 'rendering':
            return { key: 'rendering', tone: 'info', label: 'Нарізка' };
        case 'error':
            return { key: 'clip_error', tone: 'bad', label: 'Помилка нарізки' };
        default:
            return { key: 'pending', tone: 'muted', label: item.clip.url ? 'Оновлюється' : 'Очікує нарізки' };
    }
}
