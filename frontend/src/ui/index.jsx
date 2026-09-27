import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { AnimatePresence, motion } from 'framer-motion';
import { LoaderCircle, X } from 'lucide-react';
import QRCode from 'qrcode';
import { cx } from '../lib/cx';


export function Spinner({ size = 16, className = '' }) {
    return <LoaderCircle size={size} className={cx('spin', className)} />;
}

export function Button({ variant, size, icon: Icon, loading, className, children, ...props }) {
    const cls = cx(
        'btn',
        variant && `btn-${variant}`,
        size === 'sm' && 'btn-sm',
        !children && Icon && 'btn-icon',
        className,
    );
    const iconSize = size === 'sm' ? 15 : 17;
    return (
        <button type="button" className={cls} disabled={loading || props.disabled} {...props}>
            {loading ? <Spinner size={iconSize} /> : Icon && <Icon size={iconSize} strokeWidth={2.2} />}
            {children}
        </button>
    );
}

export function Field({ label, hint, error, className, children }) {
    return (
        <label className={cx('block', className)}>
            {label && <span className="label">{label}</span>}
            {children}
            {error ? (
                <span className="mt-1.5 block text-xs text-bad">{error}</span>
            ) : (
                hint && <span className="mt-1.5 block text-xs text-faint">{hint}</span>
            )}
        </label>
    );
}

export function Toggle({ checked, onChange, label, disabled }) {
    return (
        <button
            type="button"
            role="switch"
            aria-checked={checked}
            disabled={disabled}
            onClick={() => onChange(!checked)}
            className="inline-flex items-center gap-2.5 select-none disabled:opacity-40"
        >
            <span
                className={cx(
                    'relative inline-flex h-6 w-10 shrink-0 rounded-full p-0.5 transition-colors',
                    checked ? 'bg-accent' : 'bg-white/15',
                )}
            >
                <motion.span
                    layout
                    transition={{ type: 'spring', stiffness: 600, damping: 34 }}
                    className={cx('h-5 w-5 rounded-full bg-white shadow', checked && 'ml-4')}
                />
            </span>
            {label && <span className="text-sm">{label}</span>}
        </button>
    );
}

export function Segmented({ value, onChange, options, size }) {
    return (
        <div className="inline-flex rounded-xl border border-line bg-white/[0.03] p-1">
            {options.map((o) => (
                <button
                    key={o.value}
                    type="button"
                    onClick={() => onChange(o.value)}
                    className={cx(
                        'inline-flex items-center gap-1.5 rounded-lg font-semibold transition-colors',
                        size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-sm',
                        value === o.value ? 'bg-white/12 text-ink' : 'text-dim hover:text-ink',
                    )}
                >
                    {o.icon && <o.icon size={14} />}
                    {o.label}
                </button>
            ))}
        </div>
    );
}

const tones = {
    muted: 'bg-white/8 text-dim',
    ok: 'bg-ok/15 text-ok',
    warn: 'bg-warn/15 text-warn',
    bad: 'bg-bad/15 text-bad',
    info: 'bg-info/15 text-info',
    accent: 'bg-accent/15 text-accent',
};

export function Chip({ tone = 'muted', icon: Icon, children, title, className }) {
    return (
        <span className={cx('chip', tones[tone], className)} title={title}>
            {Icon && <Icon size={12} strokeWidth={2.5} />}
            {children}
        </span>
    );
}

export function ProgressBar({ value, className }) {
    return (
        <div className={cx('h-1.5 overflow-hidden rounded-full bg-white/10', className)}>
            <div
                className="h-full rounded-full bg-accent transition-[width] duration-300"
                style={{ width: `${Math.round(Math.min(Math.max(value, 0), 1) * 100)}%` }}
            />
        </div>
    );
}

// Open modals, topmost last: Escape closes only the top one.
const modalStack = [];

export function Modal({ open, onClose, title, children, footer, width = 640, bare }) {
    const closeRef = useRef(onClose);
    useEffect(() => {
        closeRef.current = onClose;
    }, [onClose]);

    useEffect(() => {
        if (!open) return;
        const token = {};
        modalStack.push(token);
        const onKey = (e) => {
            if (e.key === 'Escape' && modalStack[modalStack.length - 1] === token) closeRef.current?.();
        };
        window.addEventListener('keydown', onKey);
        const prev = document.body.style.overflow;
        document.body.style.overflow = 'hidden';
        return () => {
            modalStack.splice(modalStack.indexOf(token), 1);
            window.removeEventListener('keydown', onKey);
            document.body.style.overflow = prev;
        };
    }, [open]);

    return createPortal(
        <AnimatePresence>
            {open && (
                <motion.div
                    className="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 backdrop-blur-sm sm:items-center sm:p-4"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    onMouseDown={(e) => e.target === e.currentTarget && onClose?.()}
                >
                    <motion.div
                        role="dialog"
                        aria-modal="true"
                        className="panel flex max-h-[94vh] w-full flex-col overflow-hidden rounded-b-none shadow-2xl sm:rounded-b-[18px]"
                        style={{ maxWidth: width }}
                        initial={{ y: 30, opacity: 0 }}
                        animate={{ y: 0, opacity: 1 }}
                        exit={{ y: 20, opacity: 0 }}
                        transition={{ type: 'spring', stiffness: 380, damping: 34 }}
                    >
                        {!bare && (
                            <div className="flex items-center justify-between gap-3 border-b border-line px-5 py-3.5">
                                <h3 className="m-0 text-base font-bold">{title}</h3>
                                <Button variant="ghost" size="sm" icon={X} onClick={onClose} aria-label="Закрити" />
                            </div>
                        )}
                        <div className="min-h-0 flex-1 overflow-auto p-5">{children}</div>
                        {footer && (
                            <div className="flex flex-wrap justify-end gap-2 border-t border-line px-5 py-3.5">{footer}</div>
                        )}
                    </motion.div>
                </motion.div>
            )}
        </AnimatePresence>,
        document.body,
    );
}

export function Empty({ icon: Icon, title, children, action }) {
    return (
        <div className="flex flex-col items-center justify-center px-6 py-14 text-center">
            {Icon && (
                <div className="mb-4 grid h-14 w-14 place-items-center rounded-2xl bg-white/5 text-dim">
                    <Icon size={26} />
                </div>
            )}
            <div className="font-bold">{title}</div>
            {children && <div className="mt-1.5 max-w-sm text-sm text-dim">{children}</div>}
            {action && <div className="mt-5">{action}</div>}
        </div>
    );
}

export function QR({ value, size = 180, className, light = '#ffffff', dark = '#0d0b12' }) {
    const [svg, setSvg] = useState('');
    useEffect(() => {
        let alive = true;
        QRCode.toString(value, { type: 'svg', margin: 1, errorCorrectionLevel: 'M', color: { dark, light } })
            .then((s) => alive && setSvg(s))
            .catch(() => {});
        return () => {
            alive = false;
        };
    }, [value, dark, light]);
    return (
        <div
            className={cx('overflow-hidden rounded-xl [&>svg]:block [&>svg]:h-full [&>svg]:w-full', className)}
            style={{ width: size, height: size, background: light }}
            dangerouslySetInnerHTML={{ __html: svg }}
        />
    );
}

// InlineEdit shows text that turns into an input on click; saves on Enter/blur.
export function InlineEdit({ value, onSave, placeholder, className, inputClassName, maxLength = 300 }) {
    const [editing, setEditing] = useState(false);
    const [draft, setDraft] = useState(value);
    const ref = useRef(null);
    useEffect(() => {
        if (editing) ref.current?.select();
    }, [editing]);

    const commit = () => {
        setEditing(false);
        const v = draft.trim();
        if (v !== (value || '').trim()) onSave(v);
    };
    if (editing) {
        return (
            <input
                ref={ref}
                className={cx('field py-1', inputClassName)}
                value={draft}
                maxLength={maxLength}
                placeholder={placeholder}
                onChange={(e) => setDraft(e.target.value)}
                onBlur={commit}
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => {
                    if (e.key === 'Enter') commit();
                    if (e.key === 'Escape') {
                        setDraft(value);
                        setEditing(false);
                    }
                }}
            />
        );
    }
    return (
        <button
            type="button"
            className={cx('min-w-0 truncate text-left hover:text-accent-2', !value && 'italic text-faint', className)}
            onClick={(e) => {
                e.stopPropagation();
                setDraft(value || '');
                setEditing(true);
            }}
            title="Натисніть, щоб змінити"
        >
            {value || placeholder}
        </button>
    );
}
