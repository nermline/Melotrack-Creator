import { useParams, Link } from 'react-router-dom';
import { useGameWS, useProject } from '../game/useGame';
import GameScreen from '../game/GameScreen';
import GameRemote from '../game/GameRemote';

// Комбо-режим для редактора/адміна: екран (превʼю) і пульт на одній сторінці,
// щоб бачити, як виглядатиме показ. Одне зʼєднання роль remote керує і відображає.
export default function PresentView() {
    const { pid } = useParams();
    const { cats } = useProject(pid);
    const { state, offsetRef, send } = useGameWS(pid, 'remote');

    return (
        <div style={{ padding: 16, background: '#15151b', minHeight: '100vh', color: '#eee' }}>
            <p style={{ margin: '0 0 8px' }}>
                <Link to={`/projects/${pid}`} style={{ color: '#8af' }}>← Проєкт</Link>
            </p>
            <h1 style={{ fontSize: 18, marginTop: 0 }}>Превʼю показу (екран + пульт)</h1>

            <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap', alignItems: 'flex-start' }}>
                <div style={{ width: 640, maxWidth: '100%', aspectRatio: '16 / 9', border: '1px solid #333' }}>
                    <GameScreen pid={pid} cats={cats} state={state} offsetRef={offsetRef} />
                </div>
                <div style={{ flex: '1 1 320px', minWidth: 300 }}>
                    <GameRemote state={state} offsetRef={offsetRef} send={send} />
                </div>
            </div>
        </div>
    );
}
