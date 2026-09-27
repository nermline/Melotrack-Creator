import { lazy, Suspense, useEffect, useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { api, setUnauthorizedHandler } from './lib/api';
import { FeedbackProvider } from './ui/feedback';
import { Spinner } from './ui';
import Login from './pages/Login';
import Projects from './pages/Projects';
import ProjectPage from './project/ProjectPage';

// The show pages pull in the presentation themes and fonts; load them on demand.
const Screen = lazy(() => import('./pages/Screen'));
const Remote = lazy(() => import('./pages/Remote'));
const Preview = lazy(() => import('./pages/Preview'));
const Join = lazy(() => import('./pages/Join'));

function Loading() {
    return (
        <div className="grid min-h-screen place-items-center text-dim">
            <Spinner size={22} />
        </div>
    );
}

// Private checks the session once and redirects to the login page when needed.
function Private({ children }) {
    const [state, setState] = useState('checking');
    const location = useLocation();
    const navigate = useNavigate();

    useEffect(() => {
        setUnauthorizedHandler(() => {
            const next = location.pathname + location.search;
            navigate(`/login?next=${encodeURIComponent(next)}`, { replace: true });
        });
    }, [location, navigate]);

    useEffect(() => {
        let alive = true;
        api.get('/api/session')
            .then(() => alive && setState('ok'))
            .catch(() => alive && setState('out'));
        return () => {
            alive = false;
        };
    }, []);

    if (state === 'checking') return <Loading />;
    if (state === 'out') {
        const next = location.pathname + location.search;
        return <Navigate to={`/login?next=${encodeURIComponent(next)}`} replace />;
    }
    return children;
}

export default function App() {
    return (
        <BrowserRouter>
            <FeedbackProvider>
                <Suspense fallback={<Loading />}>
                    <Routes>
                        <Route path="/login" element={<Login />} />
                        <Route path="/join/:code" element={<Join />} />
                        <Route path="/" element={<Private><Projects /></Private>} />
                        <Route path="/p/:pid/screen" element={<Private><Screen /></Private>} />
                        <Route path="/p/:pid/remote" element={<Private><Remote /></Private>} />
                        <Route path="/p/:pid/preview" element={<Private><Preview /></Private>} />
                        <Route path="/p/:pid/:tab?" element={<Private><ProjectPage /></Private>} />
                        <Route path="*" element={<Navigate to="/" replace />} />
                    </Routes>
                </Suspense>
            </FeedbackProvider>
        </BrowserRouter>
    );
}
