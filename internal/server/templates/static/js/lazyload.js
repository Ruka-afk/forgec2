(function() {
    'use strict';

    function initLazyLoad() {
        if ('IntersectionObserver' in window) {
            const imageObserver = new IntersectionObserver((entries, observer) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        const img = entry.target;
                        loadImage(img);
                        observer.unobserve(img);
                    }
                });
            }, {
                rootMargin: '50px 0px',
                threshold: 0.01
            });

            document.querySelectorAll('img[data-src]').forEach(img => {
                imageObserver.observe(img);
            });
        } else {
            document.querySelectorAll('img[data-src]').forEach(img => {
                loadImage(img);
            });
        }
    }

    function loadImage(img) {
        const src = img.getAttribute('data-src');
        if (!src) return;

        const tempImg = new Image();
        tempImg.onload = function() {
            img.src = src;
            img.classList.add('loaded');
            img.removeAttribute('data-src');
        };
        tempImg.onerror = function() {
            img.classList.add('loaded');
        };
        tempImg.src = src;
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initLazyLoad);
    } else {
        initLazyLoad();
    }

    window.LazyLoad = {
        init: initLazyLoad,
        refresh: initLazyLoad
    };
})();
