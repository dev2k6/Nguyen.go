// nguyen_bridge.js - Nguyen.go browser bridge
// Purpose: Load WASM, bootstrap app, HMR via SSE, prefetch observer, image enhancement
// Target size: < 8KB

(function () {
    'use strict';

    var Go = window.Go || function () {
        this.importObject = {
            gojs: {},
            go: { debug: function (v) { console.log(v); }, runtime: {}, syscall: {} },
            env: {}
        };
    };

    // ========== Runtime Capabilities ==========

    var __nguyenCapabilities = null;

    async function fetchCapabilities() {
        if (__nguyenCapabilities) return __nguyenCapabilities;
        try {
            var resp = await fetch('/_nguyen/runtime');
            if (resp.ok) {
                __nguyenCapabilities = await resp.json();
            }
        } catch (e) {
            __nguyenCapabilities = { chunkMode: false };
        }
        return __nguyenCapabilities;
    }

    // ========== WASM Module Loader (per-route code splitting) ==========

    var __nguyenManifest = null;
    var __nguyenSharedModule = null;
    var __nguyenLoadedChunks = Object.create(null);
    var __nguyenChunkCache = Object.create(null);

    var __nguyenPrefetchedRoutes = Object.create(null);
    var __nguyenPendingPrefetch = Object.create(null);
    var __nguyenLastPrefetchAt = 0;
    var __nguyenPrefetchIntervalMs = 120;

    function normalizeRoutePath(input) {
        if (!input || typeof input !== 'string') return null;
        try {
            var url = new URL(input, window.location.origin);
            if (url.origin !== window.location.origin) return null;
            var path = url.pathname || '/';
            path = path.replace(/\/+/g, '/');
            if (path.length > 1 && path.endsWith('/')) {
                path = path.slice(0, -1);
            }
            return path || '/';
        } catch (e) {
            return null;
        }
    }

    async function loadManifest() {
        if (__nguyenManifest) return __nguyenManifest;
        try {
            var resp = await fetch('/chunks/manifest.json');
            if (!resp.ok) return null;
            __nguyenManifest = await resp.json();
            return __nguyenManifest;
        } catch (e) {
            return null;
        }
    }

    function matchRoutePattern(pattern, path) {
        if (!pattern || !path) return false;
        if (pattern === path || pattern === '/*') return true;

        var patternSegs = pattern.split('/').filter(Boolean);
        var pathSegs = path.split('/').filter(Boolean);

        var i = 0;
        var j = 0;

        while (i < patternSegs.length) {
            var p = patternSegs[i];

            if (p === '*') {
                return true;
            }

            if (j >= pathSegs.length) {
                return false;
            }

            if (p.charAt(0) === ':') {
                i++;
                j++;
                continue;
            }

            if (p !== pathSegs[j]) {
                return false;
            }

            i++;
            j++;
        }

        return j === pathSegs.length;
    }

    function findChunkForPath(routePath, manifest) {
        if (!manifest || !routePath) return null;

        if (manifest[routePath]) {
            return manifest[routePath];
        }

        var candidates = Object.keys(manifest);
        var bestPattern = null;
        var bestScore = -1;

        for (var i = 0; i < candidates.length; i++) {
            var pattern = candidates[i];
            if (!matchRoutePattern(pattern, routePath)) continue;

            var score = 0;
            var segs = pattern.split('/').filter(Boolean);
            for (var s = 0; s < segs.length; s++) {
                var seg = segs[s];
                if (seg === '*') {
                    score += 0;
                } else if (seg.charAt(0) === ':') {
                    score += 1;
                } else {
                    score += 10;
                }
            }

            if (score > bestScore) {
                bestScore = score;
                bestPattern = pattern;
            }
        }

        if (bestPattern) {
            return manifest[bestPattern];
        }

        return manifest['/'] || null;
    }

    async function loadSharedWasm() {
        if (__nguyenSharedModule) return __nguyenSharedModule;
        try {
            var go = new Go();
            var resp = await fetch('/chunks/shared.wasm');
            if (!resp.ok) throw new Error('shared.wasm not found');
            var buffer = await resp.arrayBuffer();
            var module = await WebAssembly.compile(buffer);
            __nguyenSharedModule = await WebAssembly.instantiate(module, go.importObject);
            go.run(__nguyenSharedModule);
            return __nguyenSharedModule;
        } catch (e) {
            console.warn('[Nguyen.go] Shared WASM not found, using single-wasm mode');
            return null;
        }
    }

    window.__nguyenLoadRoute = async function (routePath) {
        var caps = await fetchCapabilities();
        if (!caps.chunkMode) return null;

        var normalized = normalizeRoutePath(routePath);
        if (!normalized) return null;

        var manifest = await loadManifest();
        if (!manifest) return null;

        var chunkName = findChunkForPath(normalized, manifest);
        if (!chunkName) return null;

        if (__nguyenLoadedChunks[chunkName]) return __nguyenChunkCache[chunkName];

        try {
            var go = new Go();
            var resp = await fetch('/chunks/' + chunkName);
            if (!resp.ok) return null;
            var buffer = await resp.arrayBuffer();
            var module = await WebAssembly.compile(buffer);
            var instance = await WebAssembly.instantiate(module, go.importObject);
            go.run(instance);

            __nguyenLoadedChunks[chunkName] = true;
            __nguyenChunkCache[chunkName] = instance.exports;
            console.log('[Nguyen.go] Loaded chunk:', chunkName);
            return instance.exports;
        } catch (e) {
            console.error('[Nguyen.go] Failed to load chunk:', chunkName, e);
            return null;
        }
    };

    function prefetchRoute(routePath) {
        if (!__nguyenCapabilities || !__nguyenCapabilities.chunkMode) return;

        var normalized = normalizeRoutePath(routePath);
        if (!normalized) return;
        if (__nguyenPrefetchedRoutes[normalized]) return;
        if (__nguyenPendingPrefetch[normalized]) return;

        var now = Date.now();
        if (now - __nguyenLastPrefetchAt < __nguyenPrefetchIntervalMs) {
            return;
        }
        __nguyenLastPrefetchAt = now;

        __nguyenPendingPrefetch[normalized] = true;
        window.__nguyenLoadRoute(normalized)
            .then(function () {
                __nguyenPrefetchedRoutes[normalized] = true;
            })
            .catch(function () {})
            .finally(function () {
                delete __nguyenPendingPrefetch[normalized];
            });
    }

    // ========== PWA Registration ==========

    function registerServiceWorker() {
        if (typeof navigator !== 'undefined' && 'serviceWorker' in navigator) {
            window.addEventListener('load', function () {
                fetch('/sw.js', { method: 'HEAD' })
                    .then(function (resp) {
                        if (!resp.ok) return null;
                        return navigator.serviceWorker.register('/sw.js');
                    })
                    .then(function (reg) {
                        if (!reg) return;
                        console.log('[Nguyen.go PWA] SW registered:', reg.scope);
                    })
                    .catch(function (err) {
                        console.warn('[Nguyen.go PWA] SW registration failed:', err);
                    });
            });
        }
    }

    // ========== Bootstrap ==========

    async function bootstrap() {
        // Check if SSR already rendered the page with hydration markers
        var hasSSRContent = !!document.querySelector('[data-nguyen-hydrate]') ||
                            !!document.querySelector('[data-nguyen-click]') ||
                            !!document.querySelector('[data-nguyen-text]');

        if (hasSSRContent) {
            // SSR mode: page already rendered, just set up event delegation
            setupSSREventDelegation();
            console.log('[Nguyen.go] SSR hydration active');
            return;
        }

        var root = document.getElementById('app');
        if (!root) {
            console.error('[Nguyen.go] #app not found');
            return;
        }

        root.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:system-ui,sans-serif;color:#666;">'
            + '<div style="text-align:center;"><div style="font-size:48px;margin-bottom:16px;">⚡</div>'
            + '<div style="font-size:14px;color:#999;">Loading Nguyen.go...</div></div></div>';

        try {
            var caps = await fetchCapabilities();

            if (caps.chunkMode) {
                // Per-route chunk mode: manifest → shared → current route
                var manifest = await loadManifest();
                if (manifest) {
                    await loadSharedWasm();
                    var currentPath = normalizeRoutePath(window.location.pathname || '/');
                    await window.__nguyenLoadRoute(currentPath || '/');
                } else {
                    // Manifest missing despite chunkMode — fall back to single wasm
                    await loadSingleWasm();
                }
            } else {
                // Single app.wasm mode (dev or build without per-route)
                await loadSingleWasm();
            }

            console.log('[Nguyen.go] Started successfully');
        } catch (err) {
            console.error('[Nguyen.go] Init failed:', err);
            root.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:system-ui,sans-serif;">'
                + '<div style="text-align:center;color:#dc2626;"><div style="font-size:48px;margin-bottom:16px;">⚠</div>'
                + '<div>Failed to start</div></div></div>';
        }
    }

    // SSR event delegation: handles data-nguyen-{event}={handlerName} attributes
    // without requiring WASM. Used when the server rendered the page with SSR.
    // Checks both window.__nguyenHandlers and inline _handlerFns registry.
    function setupSSREventDelegation() {
        var evTypes = ['click', 'input', 'change', 'submit', 'keydown', 'keyup'];
        evTypes.forEach(function(evType) {
            document.addEventListener(evType, function(e) {
                var attr = 'data-nguyen-' + evType;
                var el = e.target.closest('[' + attr + ']');
                if (!el) return;
                var handlerName = el.getAttribute(attr);
                if (!handlerName) return;
                // Try window.__nguyenHandlers (WASM style)
                if (window.__nguyenHandlers && window.__nguyenHandlers[handlerName]) {
                    e.preventDefault();
                    window.__nguyenHandlers[handlerName](e);
                    return;
                }
                // Try _handlerFns pattern: handlerName_eventType (SSR hydration style)
                var fn = _handlerFns ? _handlerFns[handlerName] : null;
                if (!fn) {
                    fn = _handlerFns ? _handlerFns[handlerName + '_' + evType] : null;
                }
                if (fn) {
                    e.preventDefault();
                    fn(e);
                }
            });
        });
    }

    async function loadSingleWasm() {
        var go = new Go();
        var wasmModule;
        if (typeof WebAssembly.compileStreaming === 'function') {
            wasmModule = await WebAssembly.compileStreaming(fetch('/app.wasm'));
        } else {
            var resp = await fetch('/app.wasm');
            var buffer = await resp.arrayBuffer();
            wasmModule = await WebAssembly.compile(buffer);
        }
        var instance = await WebAssembly.instantiate(wasmModule, go.importObject);
        go.run(instance);
    }

    // ========== SPA Navigation ==========

    var __nguyenPartialCache = Object.create(null);
    var __nguyenPartialCacheKeys = [];
    var __nguyenPartialCacheMax = 20;
    var __nguyenNavigating = false;

    function cachePartial(path, data) {
        if (__nguyenPartialCacheKeys.length >= __nguyenPartialCacheMax) {
            var evict = __nguyenPartialCacheKeys.shift();
            delete __nguyenPartialCache[evict];
        }
        __nguyenPartialCache[path] = data;
        __nguyenPartialCacheKeys.push(path);
    }

    async function parsePartialResponse(response) {
        var buf = await response.arrayBuffer();
        if (buf.byteLength < 4) return null;
        var view = new DataView(buf);
        var metaLen = view.getUint32(0);
        if (buf.byteLength < 4 + metaLen) return null;
        var metaBytes = new Uint8Array(buf, 4, metaLen);
        var meta = JSON.parse(new TextDecoder().decode(metaBytes));
        var html = new TextDecoder().decode(new Uint8Array(buf, 4 + metaLen));
        return { meta: meta, html: html };
    }

    async function fetchPartial(path) {
        if (__nguyenPartialCache[path]) return __nguyenPartialCache[path];
        var from = window.location.pathname;
        var resp = await fetch('/_nguyen/navigate?path=' + encodeURIComponent(path) + '&from=' + encodeURIComponent(from));
        if (!resp.ok) return null;
        var data = await parsePartialResponse(resp);
        if (data) cachePartial(path, data);
        return data;
    }

    function findOutlet(depth) {
        var outlet = document.querySelector('[data-nguyen-outlet="' + depth + '"]');
        if (outlet) return outlet;
        return document.getElementById('app');
    }

    function performSwap(data) {
        var outlet = findOutlet(data.meta.outlet);
        outlet.innerHTML = data.html;

        if (data.meta.title) {
            document.title = data.meta.title;
        }

        // Re-run hydration for new content
        setupSSREventDelegation();
        enhanceImages();

        // Load WASM chunk if in chunk mode
        if (__nguyenCapabilities && __nguyenCapabilities.chunkMode) {
            window.__nguyenLoadRoute(window.location.pathname);
        }

        // Dispatch navigation event
        window.dispatchEvent(new CustomEvent('nguyen:navigate', {
            detail: { path: window.location.pathname, meta: data.meta }
        }));
    }

    async function performNavigation(path) {
        if (__nguyenNavigating) return;
        __nguyenNavigating = true;
        document.documentElement.classList.add('nguyen-navigating');

        try {
            var data = await fetchPartial(path);
            if (!data) {
                window.location.href = path;
                return;
            }
            history.pushState({}, '', path);
            performSwap(data);
        } catch (e) {
            window.location.href = path;
        } finally {
            __nguyenNavigating = false;
            document.documentElement.classList.remove('nguyen-navigating');
        }
    }

    function navigateTo(path) {
        var normalized = normalizeRoutePath(path);
        if (!normalized || normalized === window.location.pathname) return;

        if (document.startViewTransition) {
            document.startViewTransition(function () {
                return performNavigation(normalized);
            });
        } else {
            performNavigation(normalized);
        }
    }

    // Click interceptor — captures internal link clicks for SPA navigation
    document.addEventListener('click', function (e) {
        if (e.defaultPrevented) return;
        var a = e.target.closest('a[href]');
        if (!a) return;
        if (a.hasAttribute('data-no-spa')) return;
        if (a.hasAttribute('target')) return;
        if (e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;

        var href = a.getAttribute('href');
        if (!href) return;
        if (href.startsWith('http') || href.startsWith('//') || href.startsWith('#') || href.startsWith('mailto:') || href.startsWith('tel:')) return;

        e.preventDefault();
        navigateTo(href);
    });

    // Popstate — handle browser back/forward
    window.addEventListener('popstate', function () {
        var path = window.location.pathname;
        // Clear cache entry to get fresh from-context
        delete __nguyenPartialCache[path];
        performNavigation(path);
    });

    // Prefetch partial HTML on hover (extends existing WASM prefetch)
    function prefetchPartial(path) {
        var normalized = normalizeRoutePath(path);
        if (!normalized || __nguyenPartialCache[normalized]) return;
        fetchPartial(normalized).catch(function () {});
    }

    // Expose navigateTo globally for programmatic use
    window.__nguyenNavigate = navigateTo;

    // ========== Prefetch Observer ==========

    function setupPrefetchObserver() {
        document.addEventListener('mouseover', function (e) {
            var a = e.target.closest('a[href]');
            if (!a) return;
            var href = a.getAttribute('href');
            if (!href || href.startsWith('http') || href.startsWith('#')) return;
            prefetchRoute(href);
            prefetchPartial(href);
        }, { passive: true });

        if (typeof IntersectionObserver !== 'undefined') {
            var observer = new IntersectionObserver(function (entries) {
                entries.forEach(function (entry) {
                    if (!entry.isIntersecting) return;
                    var href = entry.target.getAttribute('href');
                    if (href) {
                        prefetchRoute(href);
                    }
                    observer.unobserve(entry.target);
                });
            }, { rootMargin: '200px' });

            setTimeout(function () {
                document.querySelectorAll('a[href]').forEach(function (a) {
                    observer.observe(a);
                });
            }, 1200);
        }
    }

    // ========== HMR via SSE (EventSource) ==========

    function connectHMR() {
        if (typeof EventSource === 'undefined') return;

        var sse = new EventSource('/_nguyen/hmr');

        sse.onopen = function () {
            console.log('[Nguyen.go HMR] Connected');
        };

        sse.addEventListener('css-reload', function () {
            console.log('[Nguyen.go HMR] CSS hot reload');
            var links = document.querySelectorAll('link[rel="stylesheet"]');
            links.forEach(function (link) {
                var href = link.getAttribute('href');
                if (!href) return;
                var newHref = href.replace(/[?&]v=\d+/, '') + '?v=' + Date.now();
                link.setAttribute('href', newHref);
            });
        });

        sse.addEventListener('reload', function () {
            console.log('[Nguyen.go HMR] Full reload');
            location.reload();
        });

        sse.onerror = function () {
            console.log('[Nguyen.go HMR] Disconnected, reconnecting...');
            sse.close();
            setTimeout(connectHMR, 2000);
        };
    }

    // ========== Islands Hydration ==========

    function hydrateIslands() {
        if (!__nguyenCapabilities || !__nguyenCapabilities.chunkMode) return;
        document.querySelectorAll('[data-nguyen-island-id]').forEach(function(el) {
            var islandID = el.getAttribute('data-nguyen-island-id');
            if (!islandID) return;
            var islandChunk = '/chunks/island_' + islandID + '.wasm';
            var go = new Go();
            fetch(islandChunk)
                .then(function(resp) { return resp.arrayBuffer(); })
                .then(function(buffer) { return WebAssembly.compile(buffer); })
                .then(function(mod) { return WebAssembly.instantiate(mod, go.importObject); })
                .then(function(instance) {
                    go.run(instance);
                    console.log('[Nguyen.go] Island hydrated:', islandID);
                })
                .catch(function(err) {
                    console.warn('[Nguyen.go] Island hydration failed:', islandID, err);
                });
        });
    }

    // ========== LQIP Image Enhancement ==========

    function enhanceImages() {
        document.querySelectorAll('img[data-nguyen-img]').forEach(function (img) {
            var src = img.getAttribute('data-nguyen-img');
            var blur = img.getAttribute('data-blur');
            if (blur && !img.src.startsWith('data:')) {
                img.style.backgroundImage = 'url(' + blur + ')';
                img.style.backgroundSize = 'cover';
                img.onload = function () {
                    img.style.backgroundImage = '';
                };
            }
            var w = img.getAttribute('width') || 1080;
            img.src = '/_nguyen/image?src=' + encodeURIComponent(src) + '&w=' + w + '&q=80&f=webp';
            img.removeAttribute('data-nguyen-img');
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function () {
            bootstrap().then(function () {
                setupPrefetchObserver();
                hydrateIslands();
                enhanceImages();
            });
        });
    } else {
        bootstrap().then(function () {
            setupPrefetchObserver();
            hydrateIslands();
            enhanceImages();
        });
    }

    connectHMR();
    registerServiceWorker();
})();
