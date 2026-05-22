// Допоміжні функції обрізання фото на клієнті (canvas).

function loadImage(src) {
    return new Promise((resolve, reject) => {
        const img = new Image();
        img.onload = () => resolve(img);
        img.onerror = reject;
        img.src = src;
    });
}

/**
 * Обрізає зображення за областю в пікселях (від react-easy-crop) у квадрат
 * заданого розміру і повертає JPEG-Blob.
 */
export async function getCroppedBlob(imageSrc, areaPixels, size = 800) {
    const img = await loadImage(imageSrc);
    const canvas = document.createElement('canvas');
    canvas.width = size;
    canvas.height = size;
    const ctx = canvas.getContext('2d');
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(
        img,
        areaPixels.x, areaPixels.y, areaPixels.width, areaPixels.height,
        0, 0, size, size
    );
    return new Promise((resolve) => canvas.toBlob((b) => resolve(b), 'image/jpeg', 0.9));
}

/**
 * Вписує ВСЕ зображення у квадрат size×size (без втрати інформації) з полями
 * кольору bg і повертає JPEG-Blob.
 */
export async function getFittedBlob(imageSrc, size = 800, bg = '#000') {
    const img = await loadImage(imageSrc);
    const canvas = document.createElement('canvas');
    canvas.width = size;
    canvas.height = size;
    const ctx = canvas.getContext('2d');
    ctx.fillStyle = bg;
    ctx.fillRect(0, 0, size, size);
    const scale = Math.min(size / img.width, size / img.height);
    const w = img.width * scale;
    const h = img.height * scale;
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(img, (size - w) / 2, (size - h) / 2, w, h);
    return new Promise((resolve) => canvas.toBlob((b) => resolve(b), 'image/jpeg', 0.9));
}
