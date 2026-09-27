import { useCallback, useState } from 'react';
import Cropper from 'react-easy-crop';
import { Button, Modal, Segmented } from '../ui';

function loadImage(src) {
    return new Promise((resolve, reject) => {
        const img = new Image();
        img.onload = () => resolve(img);
        img.onerror = reject;
        img.src = src;
    });
}

async function render(src, area, mode, size = 900) {
    const img = await loadImage(src);
    const canvas = document.createElement('canvas');
    canvas.width = size;
    canvas.height = size;
    const ctx = canvas.getContext('2d');
    ctx.imageSmoothingQuality = 'high';
    if (mode === 'fit') {
        ctx.fillStyle = '#000';
        ctx.fillRect(0, 0, size, size);
        const k = Math.min(size / img.width, size / img.height);
        const w = img.width * k;
        const h = img.height * k;
        ctx.drawImage(img, (size - w) / 2, (size - h) / 2, w, h);
    } else {
        ctx.drawImage(img, area.x, area.y, area.width, area.height, 0, 0, size, size);
    }
    return new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.9));
}

// ImageCropModal turns any picture (a file or the YouTube thumbnail) into the
// square answer card image.
export default function ImageCropModal({ open, src, onCancel, onConfirm }) {
    return (
        <Modal open={open} onClose={onCancel} title="Фото для відповіді" width={560}>
            <CropBody key={src} src={src} onCancel={onCancel} onConfirm={onConfirm} />
        </Modal>
    );
}

function CropBody({ src, onCancel, onConfirm }) {
    const [mode, setMode] = useState('crop');
    const [crop, setCrop] = useState({ x: 0, y: 0 });
    const [zoom, setZoom] = useState(1);
    const [area, setArea] = useState(null);
    const [busy, setBusy] = useState(false);

    const onComplete = useCallback((_, px) => setArea(px), []);

    const confirm = async () => {
        setBusy(true);
        try {
            onConfirm(await render(src, area, mode));
        } finally {
            setBusy(false);
        }
    };

    return (
        <>
            <div className="mb-3">
                <Segmented
                    value={mode}
                    onChange={setMode}
                    options={[
                        { value: 'crop', label: 'Заповнити квадрат' },
                        { value: 'fit', label: 'Вписати повністю' },
                    ]}
                />
            </div>
            <div className="relative h-[360px] overflow-hidden rounded-xl bg-black">
                {src && mode === 'crop' && (
                    <Cropper
                        image={src}
                        crop={crop}
                        zoom={zoom}
                        aspect={1}
                        onCropChange={setCrop}
                        onZoomChange={setZoom}
                        onCropComplete={onComplete}
                    />
                )}
                {src && mode === 'fit' && <img src={src} alt="" className="h-full w-full object-contain" />}
            </div>
            {mode === 'crop' && (
                <label className="mt-4 flex items-center gap-3 text-sm text-dim">
                    Масштаб
                    <input
                        type="range"
                        min={1}
                        max={4}
                        step={0.01}
                        value={zoom}
                        onChange={(e) => setZoom(Number(e.target.value))}
                        className="flex-1"
                    />
                </label>
            )}
            <div className="mt-5 flex justify-end gap-2">
                <Button variant="ghost" onClick={onCancel}>
                    Скасувати
                </Button>
                <Button variant="primary" loading={busy} disabled={mode === 'crop' && !area} onClick={confirm}>
                    Застосувати
                </Button>
            </div>
        </>
    );
}
