// Apply the theme before the page paints; all other controls work without JavaScript.
(() => {
    'use strict';

    const storageKey = 'groupie-theme';
    const root = document.documentElement;
    const systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
    let preference = null;

    try {
        const saved = localStorage.getItem(storageKey);
        if (saved === 'light' || saved === 'dark') preference = saved;
    } catch {
        // Storage can be unavailable; the toggle still works for this page.
    }

    function applyTheme() {
        const theme = preference || (systemTheme.matches ? 'dark' : 'light');
        root.dataset.theme = theme;
        const meta = document.querySelector('meta[name="theme-color"]');
        if (meta) meta.content = theme === 'dark' ? '#181c17' : '#f4f2e9';

        const toggle = document.querySelector('.theme-toggle');
        if (toggle) {
            toggle.setAttribute('aria-pressed', String(theme === 'dark'));
            toggle.title = `Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`;
        }
    }

    applyTheme();
    systemTheme.addEventListener('change', () => {
        if (!preference) applyTheme();
    });
    window.addEventListener('storage', (event) => {
        if (event.key !== storageKey && event.key !== null) return;
        preference = event.newValue === 'light' || event.newValue === 'dark' ? event.newValue : null;
        applyTheme();
    });

    document.addEventListener('DOMContentLoaded', () => {
        const toggle = document.querySelector('.theme-toggle');
        if (!toggle) return;
        applyTheme();
        toggle.hidden = false;
        toggle.addEventListener('click', () => {
            preference = root.dataset.theme === 'dark' ? 'light' : 'dark';
            try {
                localStorage.setItem(storageKey, preference);
            } catch {
                // Keep the current page usable when saving is blocked.
            }
            applyTheme();
        });
    });
})();
