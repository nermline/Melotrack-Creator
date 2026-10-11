// Theme-specific decorations for the presentation.

export function Backdrop({ theme, dim }) {
    if (theme === 'vinyl') {
        return (
            <div className={`backdrop bd-vinyl ${dim ? 'dim' : ''}`}>
                <Record className="record" />
            </div>
        );
    }
    if (theme === 'stage') {
        return (
            <div className={`backdrop bd-stage ${dim ? 'dim' : ''}`}>
                <div className="beam b1" />
                <div className="beam b2" />
                <div className="beam b3" />
                <div className="haze" />
            </div>
        );
    }
    return (
        <div className={`backdrop bd-neon ${dim ? 'dim' : ''}`}>
            <div className="stars" />
            <div className="sun" />
            <div className="floor" />
        </div>
    );
}

export function Record({ className }) {
    return (
        <svg viewBox="0 0 200 200" className={className} aria-hidden="true">
            <defs>
                <radialGradient id="rec-shine" cx="35%" cy="30%" r="80%">
                    <stop offset="0" stopColor="#3a302a" />
                    <stop offset="1" stopColor="#0d0907" />
                </radialGradient>
            </defs>
            <circle cx="100" cy="100" r="99" fill="url(#rec-shine)" />
            {[92, 84, 76, 68, 60, 52, 45].map((r) => (
                <circle key={r} cx="100" cy="100" r={r} fill="none" stroke="rgb(255 255 255 / 0.06)" strokeWidth="1" />
            ))}
            <path d="M100 8 A92 92 0 0 1 180 60" stroke="rgb(255 255 255 / 0.12)" strokeWidth="6" fill="none" strokeLinecap="round" />
            <circle cx="100" cy="100" r="34" fill="var(--a1, #f28c28)" />
            <circle cx="100" cy="100" r="34" fill="none" stroke="rgb(0 0 0 / 0.25)" strokeWidth="3" />
            <circle cx="100" cy="100" r="4" fill="#1b120c" />
        </svg>
    );
}

// What the audience sees while a song plays without its video.
export function PlayingVisual({ theme, paused, number }) {
    if (theme === 'vinyl') {
        return <Record className={`vinyl-disc ${paused ? 'paused' : ''}`} />;
    }
    if (theme === 'stage') {
        return (
            <div className={`rings ${paused ? 'paused' : ''}`}>
                <i />
                <i />
                <i />
                <span className="display" style={{ fontSize: '16cqmin', color: 'var(--a1)', textShadow: 'var(--glow)' }}>
                    {number}
                </span>
            </div>
        );
    }
    return (
        <div className={`eq ${paused ? 'paused' : ''}`}>
            {Array.from({ length: 11 }, (_, i) => (
                <span key={i} style={{ animationDuration: `${0.55 + ((i * 37) % 7) * 0.09}s`, animationDelay: `${-i * 0.13}s` }} />
            ))}
        </div>
    );
}

// Ring timer used for the countdown and the thinking pause.
export function RingTimer({ fraction, seconds, color = 'var(--a2)', size = '58cqmin' }) {
    const R = 45;
    const C = 2 * Math.PI * R;
    return (
        <div style={{ position: 'relative', width: size, height: size, maxHeight: '62cqh', maxWidth: '62cqh' }}>
            <svg viewBox="0 0 100 100" style={{ width: '100%', height: '100%', transform: 'rotate(-90deg)', overflow: 'visible' }}>
                <circle cx="50" cy="50" r={R} fill="none" stroke="rgb(255 255 255 / 0.1)" strokeWidth="4" />
                <circle
                    cx="50"
                    cy="50"
                    r={R}
                    fill="none"
                    stroke={color}
                    strokeWidth="4"
                    strokeLinecap="round"
                    strokeDasharray={C}
                    strokeDashoffset={C * (1 - fraction)}
                    style={{ filter: `drop-shadow(0 0 1.5cqmin ${color})` }}
                />
            </svg>
            <div
                className="display"
                style={{
                    position: 'absolute',
                    inset: 0,
                    display: 'grid',
                    placeItems: 'center',
                    fontSize: 'min(24cqmin, 26cqh)',
                    // Playfair (vinyl) draws old-style figures that hang below
                    // the centre; lining figures sit where the eye expects.
                    fontVariantNumeric: 'lining-nums tabular-nums',
                    fontStyle: 'normal',
                    lineHeight: 1,
                    // Centre the digit itself, not the line box with its descender space.
                    textBox: 'trim-both cap alphabetic',
                }}
            >
                {seconds}
            </div>
        </div>
    );
}
