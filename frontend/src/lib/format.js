export function fmtTime(sec, withTenths = false) {
    if (!Number.isFinite(sec) || sec < 0) sec = 0;
    const m = Math.floor(sec / 60);
    const s = sec - m * 60;
    const whole = Math.floor(s);
    const base = `${m}:${String(whole).padStart(2, '0')}`;
    return withTenths ? `${base}.${Math.floor((s - whole) * 10)}` : base;
}

export function fmtPoints(p) {
    if (p == null) return '0';
    return Number.isInteger(p) ? String(p) : p.toFixed(1).replace('.', ',');
}

export function fmtDate(iso) {
    try {
        return new Date(iso).toLocaleDateString('uk-UA', { day: 'numeric', month: 'long', year: 'numeric' });
    } catch {
        return '';
    }
}

export function plural(n, one, few, many) {
    const m10 = n % 10;
    const m100 = n % 100;
    if (m10 === 1 && m100 !== 11) return one;
    if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
    return many;
}

export const songs = (n) => `${n} ${plural(n, 'пісня', 'пісні', 'пісень')}`;
export const teams = (n) => `${n} ${plural(n, 'команда', 'команди', 'команд')}`;
export const categories = (n) => `${n} ${plural(n, 'категорія', 'категорії', 'категорій')}`;
export const points = (p) =>
    `${fmtPoints(p)} ${Number.isInteger(p) ? plural(p, 'бал', 'бали', 'балів') : 'бала'}`;

export function joinUrl(code) {
    return `${location.origin}/join/${code}`;
}
