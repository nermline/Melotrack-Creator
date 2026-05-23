import { useCallback, useEffect, useState } from 'react';
import Cropper from 'react-easy-crop';
import { Modal, Button, Spinner, Segmented } from '../ui/kit';
import { getCroppedBlob, getFittedBlob } from './cropUtils';

export default function ImageCropModal({ open, file, outputSize = 800, onCancel, onConfirm }) {
    const [src, setSrc] = useState(null);
    const [mode, setMode] = useState('crop');
    const [crop, setCrop] = useState({ x: 0, y: 0 });
    const [zoom, setZoom] = useState(1);
    const [areaPixels, setAreaPixels] = useState(null);
    const [busy, setBusy] = useState(false);

    useEffect(() => {
        if (!open || !file) { setSrc(null); return; }
        const u = URL.createObjectURL(file);
        setSrc(u);
        setMode('crop');
        setCrop({ x: 0, y: 0 });
        setZoom(1);
        setAreaPixels(null);
        return () => URL.revokeObjectURL(u);
    }, [open, file]);

    const onComplete = useCallback((_, areaPx) => setAreaPixels(areaPx), []);

    const confirm = async () => {
        if (!src) return;
        setBusy(true);
        try {
            const blob = mode === 'fit'
                ? await getFittedBlob(src, outputSize)
                : await getCroppedBlob(src, areaPixels, outputSize);
            onConfirm(blob);
        } finally { setBusy(false); }
    };

    return (
        <Modal open={open} onClose={onCancel} title="Фото відповіді" maxWidth={560}
            footer={
                <>
                    <Button variant="ghost" onClick={onCancel}>Скасувати</Button>
                    <Button variant="primary" onClick={confirm} disabled={busy || (mode === 'crop' && !areaPixels)}>
                        {busy ? <Spinner /> : 'Застосувати'}
                    </Button>
                </>
            }>
            <div className="mb-3 flex items-center justify-between gap-3 flex-wrap">
                <Segmented value={mode} onChange={setMode}
                    options={[{ value: 'crop', label: '▣ Заповнити (обрізати)' }, { value: 'fit', label: '⊡ Вмістити повністю' }]} />
                <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
                    {mode === 'crop' ? 'частина зображення обріжеться' : 'усе зображення з полями'}
                </span>
            </div>

            <div style={{ position: 'relative', width: '100%', height: 360, background: '#000',
                borderRadius: 12, overflow: 'hidden' }}>
                {src && mode === 'crop' && (
                    <Cropper image={src} crop={crop} zoom={zoom} aspect={1} showGrid restrictPosition
                        onCropChange={setCrop} onZoomChange={setZoom} onCropComplete={onComplete} />
                )}
                {src && mode === 'fit' && (
                    <img src={src} alt="" style={{ width: '100%', height: '100%', objectFit: 'contain' }} />
                )}
            </div>

            {mode === 'crop' && (
                <div className="mt-4 flex items-center gap-3">
                    <span className="text-sm" style={{ color: 'var(--color-muted)' }}>Масштаб</span>
                    <input type="range" min={1} max={4} step={0.01} value={zoom}
                        onChange={(e) => setZoom(Number(e.target.value))} style={{ flex: 1 }} />
                </div>
            )}
        </Modal>
    );
}
