import { useParams } from 'react-router-dom';
import { useGameWS, useProject } from '../game/useGame';
import GameScreen from '../game/GameScreen';

// Повноекранний показ (роль screen — лише відображення, керувати не може).
export default function ScreenView() {
    const { pid } = useParams();
    const { cats, error } = useProject(pid);
    const { state, offsetRef } = useGameWS(pid, 'screen');

    if (error) {
        return <div style={{ color: '#fff', background: '#000', height: '100vh', padding: 20 }}>{error}</div>;
    }

    return (
        <div style={{ position: 'fixed', inset: 0 }}>
            <GameScreen pid={pid} cats={cats} state={state} offsetRef={offsetRef} />
        </div>
    );
}
