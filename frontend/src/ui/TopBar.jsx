import { Link, useNavigate } from 'react-router-dom';
import { LogOut } from 'lucide-react';
import { api } from '../lib/api';
import { useSession } from '../lib/session';
import { cx } from '../lib/cx';
import { Button } from './index';
import Logo from './Logo';

export function LiveDot({ status }) {
    const map = {
        online: ['bg-ok', 'онлайн'],
        connecting: ['bg-warn animate-pulse', 'підключення…'],
        reconnecting: ['bg-warn animate-pulse', 'перепідключення…'],
        offline: ['bg-bad', 'офлайн'],
    };
    const [cls, label] = map[status] || map.offline;
    return (
        <span className="inline-flex items-center gap-1.5 text-xs text-dim" title="Живе оновлення">
            <span className={cx('h-2 w-2 rounded-full', cls)} />
            {label}
        </span>
    );
}

// Who is signed in (Google mode): a photo, or the first letter of the name.
function Me() {
    const user = useSession()?.user;
    if (!user?.email) return null;
    const label = user.name ? `${user.name} · ${user.email}` : user.email;
    return (
        <span className="hidden items-center gap-2 text-xs text-dim sm:inline-flex" title={label}>
            {user.picture ? (
                <img src={user.picture} alt="" referrerPolicy="no-referrer" className="h-7 w-7 rounded-full" />
            ) : (
                <span className="grid h-7 w-7 place-items-center rounded-full bg-accent/20 font-bold text-accent">
                    {(user.name || user.email)[0].toUpperCase()}
                </span>
            )}
            <span className="hidden max-w-[16ch] truncate lg:inline">{user.name || user.email}</span>
        </span>
    );
}

export default function TopBar({ children, right }) {
    const navigate = useNavigate();
    const logout = async () => {
        try {
            await api.post('/api/logout');
        } finally {
            navigate('/login');
        }
    };
    return (
        <header className="sticky top-0 z-30 border-b border-line bg-bg/80 backdrop-blur-md">
            <div className="mx-auto flex h-14 max-w-[1500px] items-center gap-3 px-4">
                <Link to="/" className="flex shrink-0 items-center gap-2 text-ink no-underline">
                    <Logo size={28} />
                    <span className="hidden font-display text-[15px] font-bold sm:inline">Мелотрек</span>
                </Link>
                <div className="flex min-w-0 flex-1 items-center gap-2">{children}</div>
                {right}
                <Me />
                <Button variant="ghost" size="sm" icon={LogOut} onClick={logout} title="Вийти" aria-label="Вийти" />
            </div>
        </header>
    );
}
