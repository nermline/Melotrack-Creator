import { useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

export function Button({ variant = '', size = '', className = '', children, ...props }) {
    const v = { primary: 'btn-primary', danger: 'btn-danger', ghost: 'btn-ghost' }[variant] || '';
    const s = size === 'sm' ? 'btn-sm' : '';
    return (
        <motion.button whileTap={{ scale: 0.96 }} className={`btn ${v} ${s} ${className}`} {...props}>
            {children}
        </motion.button>
    );
}

export function Glass({ className = '', strong = false, children, ...props }) {
    return <div className={`${strong ? 'glass-2' : 'glass'} ${className}`} {...props}>{children}</div>;
}

export function Badge({ tone = 'muted', children }) {
    const tones = {
        muted: { color: '#cbd5e1', bg: 'rgba(255,255,255,0.08)' },
        ok:    { color: '#a7f3c8', bg: 'rgba(68,208,123,0.16)' },
        warn:  { color: '#ffe2ad', bg: 'rgba(240,176,58,0.16)' },
        danger:{ color: '#ffd0d8', bg: 'rgba(244,96,122,0.16)' },
        accent:{ color: '#cdd0ff', bg: 'rgba(124,131,255,0.18)' },
    }[tone];
    return (
        <span style={{ color: tones.color, background: tones.bg }}
            className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium">
            {children}
        </span>
    );
}

export function Field({ label, changed, error, hint, className = '', children }) {
    return (
        <label className={`block ${className}`}>
            {label && (
                <span className="mb-1 block text-sm" style={{ color: 'var(--color-muted)' }}>
                    {label}
                    {changed && <span title="змінено, не збережено" style={{ color: 'var(--color-warn)' }}> ●</span>}
                </span>
            )}
            {children}
            {error
                ? <span className="mt-1 block text-xs" style={{ color: 'var(--color-danger)' }}>{error}</span>
                : hint && <span className="mt-1 block text-xs" style={{ color: 'var(--color-muted)' }}>{hint}</span>}
        </label>
    );
}

export function TextInput({ changed = false, error = false, className = '', ...props }) {
    return <input className={`input ${changed ? 'changed' : ''} ${error ? '!border-[var(--color-danger)]' : ''} ${className}`} {...props} />;
}

export function NumberInput({ changed = false, error = false, className = '', ...props }) {
    return <input type="number" className={`input ${changed ? 'changed' : ''} ${error ? '!border-[var(--color-danger)]' : ''} ${className}`} {...props} />;
}

export function Toggle({ checked, onChange, label, changed }) {
    return (
        <button type="button" onClick={() => onChange(!checked)}
            className="inline-flex items-center gap-2 select-none">
            <span style={{
                width: 40, height: 22, borderRadius: 999, padding: 2, display: 'inline-flex',
                background: checked ? 'linear-gradient(180deg,#8a90ff,#6c72f5)' : 'rgba(255,255,255,0.14)',
                transition: 'background .18s',
                boxShadow: changed ? '0 0 0 3px rgba(240,176,58,0.25)' : 'none',
            }}>
                <motion.span layout transition={{ type: 'spring', stiffness: 500, damping: 32 }}
                    style={{ width: 18, height: 18, borderRadius: 999, background: '#fff',
                        marginLeft: checked ? 18 : 0 }} />
            </span>
            {label && <span className="text-sm">{label}{changed && <span style={{ color: 'var(--color-warn)' }}> ●</span>}</span>}
        </button>
    );
}

export function Modal({ open, onClose, title, children, footer, maxWidth = 760 }) {
    useEffect(() => {
        if (!open) return;
        const onKey = (e) => { if (e.key === 'Escape') onClose?.(); };
        window.addEventListener('keydown', onKey);
        const prev = document.body.style.overflow;
        document.body.style.overflow = 'hidden';
        return () => { window.removeEventListener('keydown', onKey); document.body.style.overflow = prev; };
    }, [open, onClose]);

    return (
        <AnimatePresence>
            {open && (
                <motion.div
                    className="fixed inset-0 z-50 flex items-center justify-center p-4"
                    style={{ background: 'rgba(4,6,12,0.62)', backdropFilter: 'blur(4px)' }}
                    initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
                    onMouseDown={(e) => { if (e.target === e.currentTarget) onClose?.(); }}
                >
                    <motion.div className="glass w-full flex flex-col"
                        style={{ maxWidth, maxHeight: '92vh' }}
                        initial={{ opacity: 0, y: 24, scale: 0.97 }}
                        animate={{ opacity: 1, y: 0, scale: 1 }}
                        exit={{ opacity: 0, y: 16, scale: 0.98 }}
                        transition={{ type: 'spring', stiffness: 320, damping: 30 }}
                    >
                        <div className="flex items-center justify-between px-5 py-4"
                            style={{ borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
                            <h3 className="text-lg font-semibold m-0">{title}</h3>
                            <Button variant="ghost" size="sm" onClick={onClose}>✕</Button>
                        </div>
                        <div className="p-5 overflow-auto">{children}</div>
                        {footer && (
                            <div className="flex justify-end gap-2 px-5 py-4"
                                style={{ borderTop: '1px solid rgba(255,255,255,0.08)' }}>
                                {footer}
                            </div>
                        )}
                    </motion.div>
                </motion.div>
            )}
        </AnimatePresence>
    );
}

export function Segmented({ value, onChange, options }) {
    return (
        <div className="inline-flex p-1 rounded-lg"
            style={{ background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.1)' }}>
            {options.map((o) => (
                <button key={o.value} type="button" onClick={() => onChange(o.value)}
                    className="px-3 py-1.5 rounded-md text-sm transition-colors"
                    style={value === o.value
                        ? { background: 'linear-gradient(180deg,#8a90ff,#6c72f5)', color: '#fff' }
                        : { color: 'var(--color-muted)', background: 'transparent' }}>
                    {o.label}
                </button>
            ))}
        </div>
    );
}

export function Spinner({ size = 18 }) {
    return (
        <motion.span
            style={{ width: size, height: size, borderRadius: '50%', display: 'inline-block',
                border: '2px solid rgba(255,255,255,0.25)', borderTopColor: '#fff' }}
            animate={{ rotate: 360 }} transition={{ repeat: Infinity, duration: 0.8, ease: 'linear' }} />
    );
}
