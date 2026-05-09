app = {
    passwordNeeded: null,
    token: sessionStorage.getItem('access_token') || null,
    token_expires_at: parseInt(sessionStorage.getItem('access_token_expires_at')) || null,
    refreshTimer: null,

    showPage(page_id) {
        document.querySelectorAll('.page').forEach(el => el.classList.remove('active'))
        document.getElementById(page_id).classList.add('active')
    },

    parseJwt(token) {
        try {
            const base64Url = token.split('.')[1];
            const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
            const jsonPayload = decodeURIComponent(window.atob(base64).split('').map(function(c) {
                return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
            }).join(''));
            return JSON.parse(jsonPayload);
        } catch (e) {
            return null;
        }
    },

    async getLoginStatus() {
        try {
            const res = await fetch('/login/status')
            
            if (res.ok) {
                const data = await res.json()
                this.passwordNeeded = data.password_required

                const logoutBtn = document.getElementById('btn-logout');
                if (logoutBtn) {
                    if (this.passwordNeeded) {
                        logoutBtn.classList.remove('hidden');
                    } else {
                        logoutBtn.classList.add('hidden');
                    }
                }
            }
        } catch (err) {
            console.error("Cannot get login status:", err);
        }
    },

    showLoginPage() {
        if (this.passwordNeeded) {
            this.showPage('page-login')
        } else this.login()
    },

    showAdminPage() {
        this.showPage('page-admin')
        this.loadSessions()
    },

    async loadSessions() {
        if (!this.token) return;

        try {
            const res = await fetch('/admin/sessions', {
                headers: { 'Authorization': `Bearer ${this.token}` }
            });
            
            if (res.ok) {
                const sessions = await res.json();
                this.renderSessionsTable(sessions);
            } else if (res.status === 401) {
                console.warn("Unauthorized: token might be expired");
            }
        } catch (err) {
            console.error("Failed to load sessions:", err);
        }
    },

    renderSessionsTable(sessions) {
        const tbody = document.getElementById('table-sessions');
        tbody.innerHTML = ""; 

        if (!sessions || sessions.length === 0) {
            tbody.innerHTML = "<tr><td colspan='4'>No active sessions</td></tr>";
            return;
        }

        const tokenData = this.parseJwt(this.token);
        const mySessionId = tokenData ? tokenData.session_id : null;

        sessions.forEach(session => {
            const isCurrent = session.ID === mySessionId;
            const tr = document.createElement('tr');
            
            if (isCurrent) {
                tr.classList.add('current-session');
            }

            const deleteBtn = isCurrent 
                ? `<button disabled>Delete</button>`
                : `<button onclick="app.deleteSession(${session.ID})">Delete</button>`;

            tr.innerHTML = `
                <td>${session.ID} ${isCurrent ? '(Current)' : ''}</td>
                <td>${new Date(session.CreatedAt).toLocaleString('uk-UA')}</td>
                <td>${new Date(session.ExpiresAt).toLocaleString('uk-UA')}</td>
                <td>${deleteBtn}</td>
            `;
            tbody.appendChild(tr);
        });
    },

    async deleteSession(id) {
        try {
            const res = await fetch(`/admin/sessions/delete/${id}`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${this.token}` }
            });
            
            if (res.ok) {
                this.loadSessions(); 
            }
        } catch (err) {
            console.error("Delete error:", err);
        }
    },

    async refreshToken() {
        try {
            const res = await fetch('/refresh', {
                method: 'POST', 
            });

            if (res.ok) {
                const data = await res.json();
                this.saveToken(data.access_token, data.expires_in);
                return true;
            }
            return false;
        } catch (err) {
            console.error("Refresh error", err);
            return false;
        }
    },

    saveToken(access_token, expires_in) {
        this.token = access_token
        this.token_expires_at = Date.now() + (expires_in * 1000) 

        sessionStorage.setItem('access_token', this.token)
        sessionStorage.setItem('access_token_expires_at', this.token_expires_at.toString())

        this.scheduleTokenRefresh()
    },

    scheduleTokenRefresh() {
        if (this.refreshTimer) clearTimeout(this.refreshTimer)

        if (!this.token_expires_at) return

        const timeLeft = this.token_expires_at - Date.now();
        const buffer = 60000; // 60 seconds

        if (timeLeft <= buffer) {
            this.refreshToken();
        } else {
            const timeUntilRefresh = timeLeft - buffer;
            this.refreshTimer = setTimeout(() => {
                this.refreshToken();
            }, timeUntilRefresh);
        }
    },

    async login() {
        const passwordInput = document.getElementById('input-password');
        const password = passwordInput ? passwordInput.value : "";
        
        const errorEl = document.getElementById('error-login');
        if (errorEl) errorEl.innerText = "";

        try {
            const res = await fetch('/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ password: password })
            })

            if (res.ok) {
                const data = await res.json()

                this.saveToken(data.access_token, data.expires_in);  
                
                if (passwordInput) passwordInput.value = "";
                this.showAdminPage()  
            } else {
                if (errorEl) errorEl.innerText = "Bad password";
            }
        } catch (err) {
            if (errorEl) errorEl.innerText = "Internal error";
        }
    },

    async logout() {
        if (this.token) {
            try {
                await fetch('/logout', {
                    method: 'POST',
                    headers: {
                        'Authorization': `Bearer ${this.token}`
                    }
                });
            } catch (err) {
                console.error("Logout error: ", err);
            }
        }

        this.token = null;
        this.token_expires_at = null;

        sessionStorage.removeItem('access_token');
        sessionStorage.removeItem('token_expires_at');

        if (this.refreshTimer) {
            clearTimeout(this.refreshTimer);
            this.refreshTimer = null;
        }

        this.showLoginPage();
    },

    async init() {
        await this.getLoginStatus()

        if (this.token && this.token_expires_at) {
            const isExpired = Date.now() >= this.token_expires_at;
            
            if (isExpired) {
                const refreshed = await this.refreshToken();
                if (refreshed) {
                    this.showAdminPage();
                } else {
                    this.showLoginPage();
                }
            } else {
                this.scheduleTokenRefresh();
                this.showAdminPage();
            }
        } else {
            const refreshed = await this.refreshToken();

            if (refreshed) {
                this.showAdminPage();
            } else {
                this.showLoginPage();
            }
        }
    },
}

document.addEventListener("DOMContentLoaded", () => app.init())

