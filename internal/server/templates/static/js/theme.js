// Theme Manager - Dark/Light Mode Toggle
const ThemeManager = (function() {
    
    const STORAGE_KEY = 'forgec2_theme';
    
    // Default themes
    const themes = {
        dark: {
            name: '暗黑模式',
            variables: {
                '--bg-primary': '#0F172A',
                '--bg-secondary': '#1E293B',
                '--bg-tertiary': '#334155',
                '--text-primary': '#F1F5F9',
                '--text-secondary': '#94A3B8',
                '--text-tertiary': '#64748B',
                '--border-color': '#334155',
                '--accent-color': '#4F46E5',
                '--success-color': '#10B981',
                '--danger-color': '#EF4444',
                '--warning-color': '#F59E0B',
                '--info-color': '#3B82F6',
            }
        },
        light: {
            name: '明亮模式',
            variables: {
                '--bg-primary': '#FFFFFF',
                '--bg-secondary': '#F8FAFC',
                '--bg-tertiary': '#E2E8F0',
                '--text-primary': '#0F172A',
                '--text-secondary': '#475569',
                '--text-tertiary': '#64748B',
                '--border-color': '#CBD5E1',
                '--accent-color': '#4F46E5',
                '--success-color': '#10B981',
                '--danger-color': '#EF4444',
                '--warning-color': '#F59E0B',
                '--info-color': '#3B82F6',
            }
        },
        hacker: {
            name: '黑客模式',
            variables: {
                '--bg-primary': '#000000',
                '--bg-secondary': '#0A0A0A',
                '--bg-tertiary': '#1A1A1A',
                '--text-primary': '#00FF00',
                '--text-secondary': '#00CC00',
                '--text-tertiary': '#009900',
                '--border-color': '#00FF00',
                '--accent-color': '#00FF00',
                '--success-color': '#00FF00',
                '--danger-color': '#FF0000',
                '--warning-color': '#FFFF00',
                '--info-color': '#00FFFF',
            }
        }
    };
    
    let currentTheme = 'dark';
    
    // Get current theme
    function getCurrent() {
        return currentTheme;
    }
    
    // Get theme name
    function getThemeName(theme) {
        return themes[theme]?.name || 'Unknown';
    }
    
    // Apply theme
    function applyTheme(theme) {
        if (!themes[theme]) {
            console.error(`[ThemeManager] Theme '${theme}' not found`);
            return;
        }
        
        const themeData = themes[theme];
        const root = document.documentElement;
        
        // Apply CSS variables
        Object.entries(themeData.variables).forEach(([key, value]) => {
            root.style.setProperty(key, value);
        });
        
        // Update body class
        document.body.className = document.body.className.replace(/theme-\w+/g, '');
        document.body.classList.add(`theme-${theme}`);
        
        // Save to localStorage
        localStorage.setItem(STORAGE_KEY, theme);
        
        currentTheme = theme;
        
        // Dispatch event
        window.dispatchEvent(new CustomEvent('theme-changed', {
            detail: { theme, name: themeData.name }
        }));
    }
    
    // Toggle between dark and light
    function toggle() {
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        applyTheme(newTheme);
    }
    
    // Load saved theme
    function loadSaved() {
        const saved = localStorage.getItem(STORAGE_KEY);
        if (saved && themes[saved]) {
            applyTheme(saved);
        } else {
            // Default to dark mode
            applyTheme('dark');
        }
    }
    
    // Get all available themes
    function getThemes() {
        return Object.keys(themes).map(key => ({
            id: key,
            name: themes[key].name
        }));
    }
    
    // Create theme toggle button
    function createToggleButton() {
        const btn = document.createElement('button');
        btn.className = 'theme-toggle-btn p-2 rounded-lg hover:bg-slate-700 transition-colors';
        btn.title = '切换主题';
        
        updateToggleIcon(btn);
        
        btn.addEventListener('click', () => {
            toggle();
            updateToggleIcon(btn);
        });
        
        // Listen for theme changes
        window.addEventListener('theme-changed', () => {
            updateToggleIcon(btn);
        });
        
        return btn;
    }
    
    function updateToggleIcon(btn) {
        const icons = {
            dark: 'fa-moon',
            light: 'fa-sun',
            hacker: 'fa-terminal'
        };
        
        btn.innerHTML = `<i class="fa-solid ${icons[currentTheme] || 'fa-moon'}"></i>`;
    }
    
    // Create theme selector dropdown
    function createSelector() {
        const container = document.createElement('div');
        container.className = 'theme-selector relative';
        
        container.innerHTML = `
            <button class="theme-selector-btn p-2 rounded-lg hover:bg-slate-700 transition-colors" title="选择主题">
                <i class="fa-solid fa-palette"></i>
            </button>
            <div class="theme-dropdown hidden absolute right-0 mt-2 w-48 bg-slate-800 border border-slate-700 rounded-lg shadow-lg overflow-hidden">
                ${getThemes().map(theme => `
                    <button class="theme-option w-full text-left px-4 py-2 hover:bg-slate-700 transition-colors flex items-center justify-between"
                            data-theme="${theme.id}">
                        <span class="text-sm text-slate-300">${theme.name}</span>
                        <i class="fa-solid fa-check text-green-500 hidden" data-check="${theme.id}"></i>
                    </button>
                `).join('')}
            </div>
        `;
        
        const btn = container.querySelector('.theme-selector-btn');
        const dropdown = container.querySelector('.theme-dropdown');
        
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            dropdown.classList.toggle('hidden');
            updateCheckmarks();
        });
        
        container.querySelectorAll('.theme-option').forEach(option => {
            option.addEventListener('click', (e) => {
                e.stopPropagation();
                const theme = option.dataset.theme;
                applyTheme(theme);
                dropdown.classList.add('hidden');
                updateCheckmarks();
            });
        });
        
        // Close on outside click
        document.addEventListener('click', () => {
            dropdown.classList.add('hidden');
        });
        
        function updateCheckmarks() {
            container.querySelectorAll('[data-check]').forEach(check => {
                if (check.dataset.check === currentTheme) {
                    check.classList.remove('hidden');
                } else {
                    check.classList.add('hidden');
                }
            });
        }
        
        updateCheckmarks();
        
        return container;
    }
    
    // Auto-detect system preference
    function detectSystemPreference() {
        if (window.matchMedia) {
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            return prefersDark ? 'dark' : 'light';
        }
        return 'dark';
    }
    
    // Initialize
    function init() {
        loadSaved();
        
    }
    
    // Cleanup
    function destroy() {
        // Remove event listeners if needed
    }
    
    return {
        init,
        destroy,
        getCurrent,
        getThemeName,
        applyTheme,
        toggle,
        getThemes,
        createToggleButton,
        createSelector,
        detectSystemPreference
    };
})();

// Accessibility Enhancements
const Accessibility = (function() {
    
    // Focus management
    function trapFocus(element) {
        const focusableElements = element.querySelectorAll(
            'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        
        const firstFocusable = focusableElements[0];
        const lastFocusable = focusableElements[focusableElements.length - 1];
        
        element.addEventListener('keydown', (e) => {
            if (e.key === 'Tab') {
                if (e.shiftKey) {
                    if (document.activeElement === firstFocusable) {
                        lastFocusable.focus();
                        e.preventDefault();
                    }
                } else {
                    if (document.activeElement === lastFocusable) {
                        firstFocusable.focus();
                        e.preventDefault();
                    }
                }
            }
        });
    }
    
    // Announce to screen readers
    function announce(message, priority = 'polite') {
        const announcer = document.createElement('div');
        announcer.setAttribute('aria-live', priority);
        announcer.setAttribute('aria-atomic', 'true');
        announcer.className = 'sr-only';
        announcer.textContent = message;
        
        document.body.appendChild(announcer);
        
        setTimeout(() => {
            announcer.remove();
        }, 1000);
    }
    
    // High contrast mode
    function enableHighContrast() {
        document.body.classList.add('high-contrast');
        localStorage.setItem('forgec2_high_contrast', 'true');
    }
    
    function disableHighContrast() {
        document.body.classList.remove('high-contrast');
        localStorage.setItem('forgec2_high_contrast', 'false');
    }
    
    function toggleHighContrast() {
        if (document.body.classList.contains('high-contrast')) {
            disableHighContrast();
        } else {
            enableHighContrast();
        }
    }
    
    // Font size adjustment
    function increaseFontSize() {
        const current = parseFloat(getComputedStyle(document.documentElement).fontSize);
        document.documentElement.style.fontSize = (current + 2) + 'px';
        localStorage.setItem('forgec2_font_size', current + 2);
    }
    
    function decreaseFontSize() {
        const current = parseFloat(getComputedStyle(document.documentElement).fontSize);
        document.documentElement.style.fontSize = Math.max(12, current - 2) + 'px';
        localStorage.setItem('forgec2_font_size', Math.max(12, current - 2));
    }
    
    function resetFontSize() {
        document.documentElement.style.fontSize = '16px';
        localStorage.setItem('forgec2_font_size', 16);
    }
    
    // Load saved preferences
    function loadPreferences() {
        if (localStorage.getItem('forgec2_high_contrast') === 'true') {
            enableHighContrast();
        }
        
        const savedFontSize = localStorage.getItem('forgec2_font_size');
        if (savedFontSize) {
            document.documentElement.style.fontSize = savedFontSize + 'px';
        }
    }
    
    function init() {
        loadPreferences();
        
        // Register accessibility shortcuts
        KeyboardShortcuts.register('alt+h', '切换高对比度', toggleHighContrast);
        KeyboardShortcuts.register('alt+plus', '增大字体', increaseFontSize);
        KeyboardShortcuts.register('alt+minus', '减小字体', decreaseFontSize);
        KeyboardShortcuts.register('alt+0', '重置字体', resetFontSize);
        
    }
    
    return {
        init,
        trapFocus,
        announce,
        enableHighContrast,
        disableHighContrast,
        toggleHighContrast,
        increaseFontSize,
        decreaseFontSize,
        resetFontSize
    };
})();

// Export to global scope
window.ThemeManager = ThemeManager;
window.Accessibility = Accessibility;

// Auto-initialize
// Disabled to avoid conflicts with existing Tailwind styles
// if (document.readyState === 'loading') {
//     document.addEventListener('DOMContentLoaded', () => {
//         ThemeManager.init();
//         Accessibility.init();
//     });
// } else {
//     ThemeManager.init();
//     Accessibility.init();
// }
