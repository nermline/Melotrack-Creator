// A vinyl record with an equaliser for a label.
export default function Logo({ size = 32 }) {
    return (
        <svg width={size} height={size} viewBox="0 0 48 48" aria-hidden="true">
            <defs>
                <linearGradient id="mt-logo" x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0" stopColor="#ff5a7a" />
                    <stop offset="1" stopColor="#ffc85c" />
                </linearGradient>
            </defs>
            <circle cx="24" cy="24" r="23" fill="#16131d" stroke="url(#mt-logo)" strokeWidth="2" />
            <circle cx="24" cy="24" r="17" fill="none" stroke="rgb(255 255 255 / 0.08)" />
            <circle cx="24" cy="24" r="12.5" fill="none" stroke="rgb(255 255 255 / 0.08)" />
            <circle cx="24" cy="24" r="9" fill="url(#mt-logo)" />
            <g fill="#1a0610">
                <rect x="18.6" y="22" width="2" height="5" rx="1" />
                <rect x="21.8" y="19.5" width="2" height="9" rx="1" />
                <rect x="25" y="21" width="2" height="7" rx="1" />
                <rect x="28.2" y="23" width="2" height="4" rx="1" />
            </g>
        </svg>
    );
}
