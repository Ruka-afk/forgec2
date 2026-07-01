(function() {
    'use strict';

    class VirtualList {
        constructor(options) {
            this.container = typeof options.container === 'string' 
                ? document.querySelector(options.container) 
                : options.container;
            
            if (!this.container) {
                console.warn('VirtualList: container not found');
                return;
            }

            this.items = options.items || [];
            this.itemHeight = options.itemHeight || 48;
            this.buffer = options.buffer || 8;
            this.threshold = options.threshold || 100;
            this.renderItem = options.renderItem;
            this.onScroll = options.onScroll;
            this.stickyHeader = options.stickyHeader !== false;
            this.enabled = false;
            this.scrollTop = 0;
            this._rowHeights = [];
            this._rowOffsets = [0];
            this._scrollContainer = null;
            this._spacer = null;
            this._content = null;
            this._table = null;
            this._thead = null;
            this._originalRows = [];
            this._scrollHandler = null;
            this._resizeObserver = null;
            this._rafId = null;
            this._lastRenderTime = 0;
            this._renderDelay = 16;
            
            this._init();
        }

        _init() {
            const table = this.container.querySelector('table');
            if (!table) return;
            
            const thead = table.querySelector('thead');
            const tbody = table.querySelector('tbody');
            if (!tbody) return;

            this._table = table;
            this._thead = thead;
            this._originalTbody = tbody;
            
            const rows = tbody.querySelectorAll('tr');
            this._originalRows = Array.from(rows);
            
            if (this.items.length === 0 && rows.length > 0) {
                this.items = this._originalRows.map((row, index) => ({ 
                    _element: row, 
                    _index: index,
                    _html: row.outerHTML 
                }));
            }

            this._measureInitialHeights();
        }

        _measureInitialHeights() {
            this._rowHeights = [];
            this._rowOffsets = [0];
            
            let totalHeight = 0;
            for (let i = 0; i < this.items.length; i++) {
                let h = this.itemHeight;
                if (this._originalRows[i]) {
                    h = this._originalRows[i].offsetHeight || this.itemHeight;
                }
                this._rowHeights.push(h);
                totalHeight += h;
                this._rowOffsets.push(totalHeight);
            }
            this._totalHeight = totalHeight;
        }

        setItems(items) {
            this.items = items || [];
            this._rowHeights = new Array(this.items.length).fill(this.itemHeight);
            this._rowOffsets = [0];
            for (let i = 0; i < this.items.length; i++) {
                this._rowOffsets.push(this._rowOffsets[i] + this._rowHeights[i]);
            }
            this._totalHeight = this._rowOffsets[this.items.length] || 0;
            
            if (this.enabled) {
                if (this._spacer) {
                    this._spacer.style.height = this._totalHeight + 'px';
                }
                this._render();
            }
        }

        updateItem(index, item) {
            if (index < 0 || index >= this.items.length) return;
            this.items[index] = item;
            if (this.enabled) {
                this._render();
            }
        }

        enable() {
            if (this.enabled) return;
            if (this.items.length <= this.threshold) return;
            if (!this._table) return;

            this.enabled = true;
            this._buildVirtualStructure();
            this._render();
            this._bindEvents();
            
            setTimeout(() => this.recalcHeights(), 50);
            setTimeout(() => this.recalcHeights(), 200);
        }

        disable() {
            if (!this.enabled) return;
            this.enabled = false;

            this._unbindEvents();
            
            if (this._scrollContainer && this._table) {
                const wrapper = this._scrollContainer.parentElement;
                if (wrapper && wrapper.parentElement) {
                    const tableClone = this._table.cloneNode(false);
                    if (this._thead) {
                        tableClone.appendChild(this._thead.cloneNode(true));
                    }
                    const tbody = document.createElement('tbody');
                    for (let i = 0; i < this.items.length; i++) {
                        const rowHtml = this._getRowHtml(i);
                        const temp = document.createElement('div');
                        temp.innerHTML = '<table><tbody>' + rowHtml + '</tbody></table>';
                        const row = temp.querySelector('tr');
                        if (row) tbody.appendChild(row);
                    }
                    tableClone.appendChild(tbody);
                    wrapper.parentElement.replaceChild(tableClone, wrapper);
                    this._table = tableClone;
                    this._originalTbody = tbody;
                    this._originalRows = Array.from(tbody.querySelectorAll('tr'));
                }
            }

            this._scrollContainer = null;
            this._spacer = null;
            this._content = null;
        }

        _buildVirtualStructure() {
            const table = this._table;
            const parent = table.parentElement;
            
            const wrapper = document.createElement('div');
            wrapper.className = 'virtual-table-wrapper';
            wrapper.style.position = 'relative';
            
            const scrollContainer = document.createElement('div');
            scrollContainer.className = 'virtual-scroll-container';
            scrollContainer.style.overflowY = 'auto';
            scrollContainer.style.overflowX = 'auto';
            scrollContainer.style.maxHeight = 'calc(100vh - 320px)';
            scrollContainer.style.minHeight = '300px';
            scrollContainer.style.position = 'relative';
            scrollContainer.style.webkitOverflowScrolling = 'touch';
            
            const spacer = document.createElement('div');
            spacer.className = 'virtual-spacer';
            spacer.style.position = 'relative';
            spacer.style.height = this._totalHeight + 'px';
            spacer.style.width = '100%';
            spacer.style.minWidth = '100%';
            
            const contentTable = document.createElement('table');
            contentTable.className = table.className;
            contentTable.style.position = 'absolute';
            contentTable.style.top = '0';
            contentTable.style.left = '0';
            contentTable.style.right = '0';
            contentTable.style.width = '100%';
            contentTable.style.tableLayout = 'fixed';
            
            if (this._thead) {
                const newThead = this._thead.cloneNode(true);
                if (this.stickyHeader) {
                    newThead.style.position = 'sticky';
                    newThead.style.top = '0';
                    newThead.style.zIndex = '10';
                }
                contentTable.appendChild(newThead);
                this._virtualThead = newThead;
            }
            
            const contentTbody = document.createElement('tbody');
            contentTbody.className = 'virtual-content-tbody';
            contentTable.appendChild(contentTbody);
            
            spacer.appendChild(contentTable);
            scrollContainer.appendChild(spacer);
            wrapper.appendChild(scrollContainer);
            
            parent.replaceChild(wrapper, table);
            
            this._scrollContainer = scrollContainer;
            this._spacer = spacer;
            this._content = contentTbody;
            this._virtualTable = contentTable;
            
            this._syncColumnWidths();
            this._syncHorizontalScroll();
        }

        _syncColumnWidths() {
            if (!this._thead || !this._originalRows[0]) return;
            
            const originalCells = this._originalRows[0].querySelectorAll('td, th');
            const newThs = this._virtualThead.querySelectorAll('th');
            
            if (originalCells.length && newThs.length) {
                for (let i = 0; i < Math.min(originalCells.length, newThs.length); i++) {
                    const width = originalCells[i].offsetWidth;
                    if (width > 0) {
                        newThs[i].style.width = width + 'px';
                        newThs[i].style.minWidth = width + 'px';
                    }
                }
            }
            
            if (this._virtualTable && this._table) {
                const tableWidth = this._table.offsetWidth;
                if (tableWidth > 0) {
                    this._virtualTable.style.width = tableWidth + 'px';
                    this._virtualTable.style.minWidth = tableWidth + 'px';
                    this._spacer.style.minWidth = tableWidth + 'px';
                }
            }
        }

        _syncHorizontalScroll() {
            if (!this._scrollContainer) return;
            
            this._scrollContainer.addEventListener('scroll', () => {
                const scrollLeft = this._scrollContainer.scrollLeft;
                if (this._virtualThead && this.stickyHeader) {
                    this._virtualThead.style.transform = `translateX(${-scrollLeft}px)`;
                }
            }, { passive: true });
        }

        _bindEvents() {
            this._scrollHandler = () => {
                this.scrollTop = this._scrollContainer.scrollTop;
                
                const now = Date.now();
                if (now - this._lastRenderTime < this._renderDelay) {
                    if (this._rafId) cancelAnimationFrame(this._rafId);
                    this._rafId = requestAnimationFrame(() => {
                        this._lastRenderTime = Date.now();
                        this._render();
                        this._measureVisibleHeights();
                    });
                } else {
                    this._lastRenderTime = now;
                    this._render();
                    if (this._rafId) cancelAnimationFrame(this._rafId);
                    this._rafId = requestAnimationFrame(() => {
                        this._measureVisibleHeights();
                    });
                }
                
                if (this.onScroll) {
                    this.onScroll(this.scrollTop);
                }
            };
            
            this._scrollContainer.addEventListener('scroll', this._scrollHandler, { passive: true });
            
            if (typeof ResizeObserver !== 'undefined') {
                this._resizeObserver = new ResizeObserver(() => {
                    if (this.enabled) {
                        this._syncColumnWidths();
                        this._render();
                    }
                });
                this._resizeObserver.observe(this._scrollContainer);
            }
        }

        _unbindEvents() {
            if (this._scrollHandler && this._scrollContainer) {
                this._scrollContainer.removeEventListener('scroll', this._scrollHandler);
            }
            if (this._rafId) {
                cancelAnimationFrame(this._rafId);
                this._rafId = null;
            }
            if (this._resizeObserver) {
                this._resizeObserver.disconnect();
                this._resizeObserver = null;
            }
        }

        _getRowHtml(index) {
            if (this.renderItem && this.items[index]) {
                return this.renderItem(this.items[index], index);
            }
            if (this.items[index] && this.items[index]._html) {
                return this.items[index]._html;
            }
            return '<tr><td>Row ' + index + '</td></tr>';
        }

        _getStartIndex(scrollTop) {
            let low = 0;
            let high = this.items.length - 1;
            
            while (low <= high) {
                const mid = (low + high) >> 1;
                if (this._rowOffsets[mid] <= scrollTop) {
                    low = mid + 1;
                } else {
                    high = mid - 1;
                }
            }
            
            return Math.max(0, low - 1);
        }

        _getEndIndex(startIndex, viewportHeight) {
            let accumulated = this._rowOffsets[startIndex];
            const target = this.scrollTop + viewportHeight;
            let i = startIndex;
            
            while (i < this.items.length && accumulated < target) {
                i++;
                accumulated = this._rowOffsets[i];
            }
            
            return Math.min(this.items.length, i + 1);
        }

        _render() {
            if (!this.enabled || !this._content) return;
            
            const viewportHeight = this._scrollContainer.clientHeight;
            const startIdx = Math.max(0, this._getStartIndex(this.scrollTop) - this.buffer);
            const endIdx = Math.min(this.items.length, this._getEndIndex(startIdx, viewportHeight) + this.buffer);
            
            if (startIdx === this._lastStartIdx && endIdx === this._lastEndIdx) {
                return;
            }
            this._lastStartIdx = startIdx;
            this._lastEndIdx = endIdx;
            
            const startOffset = this._rowOffsets[startIdx];
            
            const fragment = document.createDocumentFragment();
            
            for (let i = startIdx; i < endIdx; i++) {
                const rowHtml = this._getRowHtml(i);
                const temp = document.createElement('div');
                temp.innerHTML = '<table><tbody>' + rowHtml + '</tbody></table>';
                const row = temp.querySelector('tr');
                if (row) {
                    row.style.position = 'absolute';
                    row.style.top = (this._rowOffsets[i] - startOffset) + 'px';
                    row.style.left = '0';
                    row.style.right = '0';
                    row.style.display = 'table';
                    row.style.width = '100%';
                    row.style.tableLayout = 'fixed';
                    row.setAttribute('data-virtual-index', i);
                    fragment.appendChild(row);
                }
            }
            
            this._content.innerHTML = '';
            this._content.appendChild(fragment);
            this._content.style.transform = 'translateY(' + startOffset + 'px)';
            this._content.style.position = 'relative';
        }

        _measureVisibleHeights() {
            if (!this.enabled || !this._content) return;
            
            const renderedRows = this._content.querySelectorAll('tr[data-virtual-index]');
            let needsUpdate = false;
            
            renderedRows.forEach(row => {
                const idx = parseInt(row.getAttribute('data-virtual-index'));
                if (isNaN(idx)) return;
                
                const actualHeight = row.offsetHeight;
                if (actualHeight > 0 && actualHeight !== this._rowHeights[idx]) {
                    const diff = actualHeight - this._rowHeights[idx];
                    this._rowHeights[idx] = actualHeight;
                    for (let i = idx + 1; i < this._rowOffsets.length; i++) {
                        this._rowOffsets[i] += diff;
                    }
                    this._totalHeight += diff;
                    needsUpdate = true;
                }
            });
            
            if (needsUpdate) {
                if (this._spacer) {
                    this._spacer.style.height = this._totalHeight + 'px';
                }
                this._render();
            }
        }

        scrollToIndex(index, behavior) {
            if (!this.enabled || !this._scrollContainer) return;
            index = Math.max(0, Math.min(this.items.length - 1, index));
            const offset = this._rowOffsets[index];
            this._scrollContainer.scrollTo({
                top: offset,
                behavior: behavior || 'auto'
            });
        }

        scrollToTop() {
            if (this._scrollContainer) {
                this._scrollContainer.scrollTop = 0;
            }
        }

        scrollToBottom() {
            if (this._scrollContainer) {
                this._scrollContainer.scrollTop = this._totalHeight;
            }
        }

        refresh() {
            if (this.enabled) {
                this._lastStartIdx = -1;
                this._lastEndIdx = -1;
                this._render();
            }
        }

        recalcHeights() {
            this._measureVisibleHeights();
        }

        get visibleRange() {
            if (!this.enabled || !this._scrollContainer) return { start: 0, end: 0 };
            const viewportHeight = this._scrollContainer.clientHeight;
            const start = this._getStartIndex(this.scrollTop);
            const end = this._getEndIndex(start, viewportHeight);
            return { start, end };
        }

        get totalCount() {
            return this.items.length;
        }

        get scrollElement() {
            return this._scrollContainer;
        }

        destroy() {
            this.disable();
            this.items = [];
            this._rowHeights = [];
            this._rowOffsets = [0];
            this._originalRows = [];
            this._table = null;
            this._thead = null;
            this._originalTbody = null;
        }
    }

    function createVirtualList(options) {
        return new VirtualList(options);
    }

    function autoInitVirtualList(containerSelector, threshold) {
        const containers = document.querySelectorAll(containerSelector);
        const instances = [];
        
        containers.forEach(container => {
            const table = container.querySelector('table');
            if (!table) return;
            
            const tbody = table.querySelector('tbody');
            if (!tbody) return;
            
            const rows = tbody.querySelectorAll('tr');
            const thresholdVal = threshold || 100;
            
            if (rows.length > thresholdVal) {
                const vl = new VirtualList({
                    container: container,
                    threshold: thresholdVal,
                    buffer: 8
                });
                vl.enable();
                instances.push(vl);
                
                setTimeout(() => vl.recalcHeights(), 100);
            }
        });
        
        return instances;
    }

    window.VirtualList = VirtualList;
    window.createVirtualList = createVirtualList;
    window.autoInitVirtualList = autoInitVirtualList;

})();
