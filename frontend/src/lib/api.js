// Thin fetch wrapper. The session lives in an HttpOnly cookie, so there is no
// token handling here; a 401 sends the user to the login page.

export class ApiError extends Error {
    constructor(status, code, message) {
        super(message);
        this.status = status;
        this.code = code;
    }
}

let onUnauthorized = () => {};
export function setUnauthorizedHandler(fn) {
    onUnauthorized = fn;
}

async function request(method, path, body, { raw = false } = {}) {
    const init = { method, credentials: 'same-origin', headers: {} };
    if (body instanceof FormData) {
        init.body = body;
    } else if (body !== undefined) {
        init.headers['Content-Type'] = 'application/json';
        init.body = JSON.stringify(body);
    }
    let res;
    try {
        res = await fetch(path, init);
    } catch {
        throw new ApiError(0, 'network', 'Немає зʼєднання з сервером');
    }
    if (raw) return res;
    const text = await res.text();
    let data = null;
    if (text) {
        try {
            data = JSON.parse(text);
        } catch {
            data = null;
        }
    }
    if (!res.ok) {
        const err = data?.error || {};
        if (res.status === 401 && !path.startsWith('/api/login')) onUnauthorized();
        throw new ApiError(res.status, err.code || 'http', err.message || `Помилка ${res.status}`);
    }
    return data;
}

export const api = {
    get: (p) => request('GET', p),
    post: (p, b) => request('POST', p, b ?? {}),
    put: (p, b) => request('PUT', p, b),
    patch: (p, b) => request('PATCH', p, b),
    del: (p) => request('DELETE', p),
};

// Uploads with progress (fetch cannot report upload progress).
export function upload(path, formData, onProgress) {
    return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', path);
        xhr.withCredentials = true;
        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) onProgress?.(e.loaded / e.total);
        };
        xhr.onload = () => {
            let data = null;
            try {
                data = JSON.parse(xhr.responseText);
            } catch {
                /* empty */
            }
            if (xhr.status >= 200 && xhr.status < 300) resolve(data);
            else {
                if (xhr.status === 401) onUnauthorized();
                const err = data?.error || {};
                reject(new ApiError(xhr.status, err.code || 'http', err.message || `Помилка ${xhr.status}`));
            }
        };
        xhr.onerror = () => reject(new ApiError(0, 'network', 'Немає зʼєднання з сервером'));
        xhr.send(formData);
    });
}
