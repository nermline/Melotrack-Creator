import { useParams, Link } from 'react-router-dom';
import { useGameWS } from '../game/useGame';
import GameRemote from '../game/GameRemote';

// Пульт керування (роль remote — надсилає команди показу).
export default function RemoteView() {
    const { pid } = useParams();
    const { state, offsetRef, send, status } = useGameWS(pid, 'remote');

    return (
        <div style={{ padding: 16, background: '#15151b', minHeight: '100vh', color: '#eee' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h1 style={{ fontSize: 18, margin: 0 }}>🎛 Пульт керування</h1>
                <small>
                    {status === 'connected' ? '🟢' : '🔴'}{' '}
                    <Link to={`/projects/${pid}/play`} style={{ color: '#8af' }}>← вихід</Link>
                </small>
            </div>
            <div style={{ marginTop: 12 }}>
                <GameRemote state={state} offsetRef={offsetRef} send={send} />
            </div>
        </div>
    );
}
