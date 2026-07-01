const THEME_KEY = 'forgec2_theme';
const VALID_THEMES = ['light', 'dark', 'system'];

function getStoredTheme() {
    try {
        const stored = localStorage.getItem(THEME_KEY);
        return VALID_THEMES.includes(stored) ? stored : 'system';
    } catch (e) {
        return 'system';
    }
}

function setStoredTheme(theme) {
    try {
        localStorage.setItem(THEME_KEY, theme);
    } catch (e) {}
}

function getSystemTheme() {
    if (typeof window.matchMedia === 'undefined') return 'light';
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function getEffectiveTheme() {
    const stored = getStoredTheme();
    if (stored === 'system') {
        return getSystemTheme();
    }
    return stored;
}

function applyTheme(theme) {
    const html = document.documentElement;
    html.classList.remove('dark');
    
    if (theme === 'dark') {
        html.classList.add('dark');
    }
    
    updateThemeIcon(theme);
    updateThemeMenu(getStoredTheme());
}

function updateThemeIcon(theme) {
    const icons = document.querySelectorAll('.theme-icon');
    icons.forEach(icon => {
        icon.classList.remove('fa-sun', 'fa-moon', 'fa-desktop');
        if (theme === 'dark') {
            icon.classList.add('fa-moon');
        } else if (theme === 'light') {
            icon.classList.add('fa-sun');
        } else {
            icon.classList.add('fa-desktop');
        }
    });
}

function updateThemeMenu(activeTheme) {
    const buttons = document.querySelectorAll('[data-theme-action]');
    buttons.forEach(btn => {
        const theme = btn.getAttribute('data-theme-action');
        if (theme === activeTheme) {
            btn.classList.add('bg-indigo-600', 'text-white');
            btn.classList.remove('bg-slate-100', 'text-slate-700', 'dark:bg-slate-700', 'dark:text-slate-300');
        } else {
            btn.classList.remove('bg-indigo-600', 'text-white');
            btn.classList.add('bg-slate-100', 'text-slate-700', 'dark:bg-slate-700', 'dark:text-slate-300');
        }
    });
}

function setTheme(theme) {
    if (!VALID_THEMES.includes(theme)) return;
    
    setStoredTheme(theme);
    const effectiveTheme = theme === 'system' ? getSystemTheme() : theme;
    applyTheme(effectiveTheme);
}

function initTheme() {
    const effectiveTheme = getEffectiveTheme();
    applyTheme(effectiveTheme);
    
    if (typeof window.matchMedia !== 'undefined') {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
            const stored = getStoredTheme();
            if (stored === 'system') {
                applyTheme(e.matches ? 'dark' : 'light');
            }
        });
    }
}

document.addEventListener('DOMContentLoaded', function() {
    initTheme();
});

function setLanguage(lang) {
    if (!lang) return;

    const params = new URLSearchParams();
    params.append('lang', lang);

    fetch('/lang/set?' + params.toString(), {
        method: 'GET',
        credentials: 'same-origin'
    }).then(response => {
        if (response.redirected || response.ok) {
            window.location.reload();
        } else {
            return response.json().then(data => {
                if (data.success) {
                    window.location.reload();
                } else {
                    showToast(data.error || 'Failed to set language', 'error');
                }
            });
        }
    }).catch(e => {
        console.error('Language switch error:', e);
        showToast('Language switch failed', 'error');
    });
}

function initRTL() {
    const html = document.documentElement;
    const body = document.body;
    const isRTL = body.classList.contains('rtl') || html.getAttribute('dir') === 'rtl';
    if (isRTL) {
        html.setAttribute('dir', 'rtl');
        document.body.classList.add('rtl');
    }
}

document.addEventListener('DOMContentLoaded', function() {
    initTheme();
    initRTL();
});

window.setTheme = setTheme;
window.getStoredTheme = getStoredTheme;
window.getEffectiveTheme = getEffectiveTheme;
window.setLanguage = setLanguage;

if (typeof window.handleThemeSelect !== 'function') {
    window.handleThemeSelect = function(theme) {
        setTheme(theme);
        if (typeof window.closeTopBarMenus === 'function') window.closeTopBarMenus();
    };
}
if (typeof window.handleLanguageSelect !== 'function') {
    window.handleLanguageSelect = function(lang) {
        setLanguage(lang);
    };
}

// ── i18n helpers ──
function __(key) {
    if (window.__locales && window.__locales[key]) return window.__locales[key];
    return key;
}
function __t(key) {
    return __(key);
}
function __tf(key) {
    const val = __(key);
    const args = Array.prototype.slice.call(arguments, 1);
    return val.replace(/\{(\d+)\}/g, function(m, idx) { return args[idx] != null ? args[idx] : m; });
}
window.__ = __;
window.__t = __t;
window.__tf = __tf;

// ── Shared utilities ──

async function apiFetch(url, options) {
    options = options || {};
    const headers = new Headers(options.headers || {});
    if (!headers.has('Content-Type') && options.body && typeof options.body === 'string') {
        headers.set('Content-Type', 'application/json');
    }
    const resp = await fetch(url, { credentials: 'same-origin', ...options, headers });
    if (!resp.ok) {
        let msg = `HTTP ${resp.status}`;
        try { const err = await resp.json(); msg = err.error || msg; } catch (e) {}
        throw new Error(msg);
    }
    return resp.json();
}

function withLoading(el, promise) {
    if (!el) return promise;
    const orig = el.innerHTML;
    const w = el.offsetWidth;
    el.disabled = true;
    el.style.width = w + 'px';
    el.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i>';
    return Promise.resolve(promise).finally(() => {
        el.disabled = false;
        el.innerHTML = orig;
        el.style.width = '';
    });
}

function debounce(fn, ms) {
    let timer;
    return function(...args) {
        clearTimeout(timer);
        timer = setTimeout(() => fn.apply(this, args), ms || 300);
    };
}

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}
window.escapeHtml = escapeHtml;