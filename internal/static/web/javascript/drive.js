// internal/static/web/javascript/drive.js
$(document).ready(function() {
    const folderGrid = $('#folderGrid');
    const fileGrid = $('#fileGrid');
    const currentPath = $('#currentPath');
    const previewModal = $('#previewModal');
    const previewArea = $('#previewArea');
    const downloadLink = $('#downloadLink');
    const closeButton = $('.close-button');

    let currentRelPath = initialPath;
    let currentFiles = [];
    let currentFileIndex = -1;

    const loadingMessage = $('<div id="loadingMessage">Now Loading...</div>');
    $('.drive-container').append(loadingMessage);
    hideLoading();

    function showLoading() {
        loadingMessage.show();
    }

    function hideLoading() {
        loadingMessage.hide();
    }

    loadDirectory(currentRelPath);

    function loadDirectory(p) {
        showLoading();
        $.ajax({
            url: '/api/drive/list',
            method: 'GET',
            data: {path: p},
            success: function(data) {
                hideLoading();
                renderDirectory(data, p);
            },
            error: function(err) {
                hideLoading();
                console.error("Failed to load directory:", err);
                folderGrid.html('<p>Error loading directory.</p>');
                fileGrid.html('');
            }
        });
    }

    function renderDirectory(data, p) {
        folderGrid.empty();
        fileGrid.empty();

        const folders = Array.isArray(data.folders) ? data.folders.slice() : [];
        const files = Array.isArray(data.files) ? data.files.slice() : [];

        folders.sort((a, b) => a.name.localeCompare(b.name));
        files.sort((a, b) => a.name.localeCompare(b.name));

        currentFiles = files;
        currentFileIndex = -1;

        if (p === '') {
            currentPath.text('~/');
        } else {
            currentPath.text('~/' + p);
        }

        if (p !== '') {
            const parentPath = p.split('/').slice(0, -1).join('/');
            const parentDiv = $(`
                <div class="item folder parent-dir" id="parent-dir-link">
                    <div class="icon-name">
                        <span class="material-icons">folder_open</span>
                        <span>..</span>
                    </div>
                </div>
            `);
            parentDiv.click(function(){
                currentRelPath = parentPath;
                history.pushState(null, '', '/drive/' + parentPath);
                loadDirectory(parentPath);
            });
            folderGrid.append(parentDiv);
        }

        folders.forEach(item => {
            const div = $(`
                <div class="item folder" data-path="${item.path}">
                    <div class="icon-name">
                        <span class="material-icons">folder</span>
                        <span>${escapeHtml(item.name)}</span>
                    </div>
                </div>
            `);
            div.click(function(){
                currentRelPath = item.path;
                history.pushState(null, '', '/drive/' + item.path);
                loadDirectory(item.path);
            });
            folderGrid.append(div);
        });

        files.forEach((item, index) => {
            const div = $(`
                <div class="item file" data-index="${index}">
                    <div class="icon-name">
                        <span class="material-icons">insert_drive_file</span>
                        <span>${escapeHtml(item.name)}</span>
                    </div>
                </div>
            `);
            div.click(function(){
                currentFileIndex = index;
                previewFile(item);
            });
            fileGrid.append(div);
        });
    }

    function previewFile(item) {
        $.ajax({
            url: '/api/drive/preview',
            method: 'GET',
            data: {file: item.path},
            dataType: 'json',
            beforeSend: function() {
                showLoading();
            },
            success: function(resp) {
                hideLoading();
                const mime = resp.mime;
                if (mime.startsWith('text/')) {
                    loadContent(item.path, 'text', mime);
                } else if (mime.startsWith('image/')) {
                    loadContent(item.path, 'blob', mime, 'image');
                } else if (mime.startsWith('video/')) {
                    loadContent(item.path, 'blob', mime, 'video');
                } else if (mime.startsWith('audio/')) {
                    loadContent(item.path, 'blob', mime, 'audio');
                } else {
                    showModal("<p>Preview not available.</p>", item.path);
                }
            },
            error: function(err) {
                hideLoading();
                console.warn("Preview info failed, download only:", err);
                showModal("<p>Preview not available.</p>", item.path);
            }
        });
    }

    function loadContent(filePath, responseType, mime, mediaType) {
        $.ajax({
            url: '/api/drive/download',
            method: 'GET',
            data: {file: filePath},
            xhrFields: { responseType: responseType },
            success: function(data) {
                hideLoading();
                if (responseType === 'text') {
                    showModal(`<pre>${escapeHtml(data)}</pre>`, filePath);
                } else if (responseType === 'blob') {
                    let blob = new Blob([data], {type: mime});
                    let url = URL.createObjectURL(blob);
                    let mediaElem = '';
                    if (mediaType === 'image') {
                        mediaElem = `<img src="${url}" alt="${escapeHtml(filePath)}">`;
                    } else if (mediaType === 'video') {
                        mediaElem = `<video controls src="${url}"></video>`;
                    } else if (mediaType === 'audio') {
                        mediaElem = `<audio controls src="${url}"></audio>`;
                    }
                    showModal(mediaElem, filePath);
                }
            },
            error: function(err) {
                hideLoading();
                console.error("Content load failed:", err);
                showModal("<p>Failed to load content.</p>", filePath);
            }
        });
    }

    function showModal(contentHtml, filePath) {
        downloadLink.html(`<a href="/api/drive/download?file=${encodeURIComponent(filePath)}" class="material-icons" title="Download">download</a>`);
        previewArea.html(contentHtml);
        previewModal.addClass('active');
    }

    function closeModal() {
        previewModal.removeClass('active');
        previewArea.html('');
        downloadLink.html('');
        currentFileIndex = -1;
    }

    closeButton.click(function() {
        closeModal();
    });

    $(document).keydown(function(e) {
        if (previewModal.hasClass('active')) {
            if (e.key === "ArrowLeft") {
                navigatePreview(-1);
            } else if (e.key === "ArrowRight") {
                navigatePreview(1);
            } else if (e.key === "Escape") {
                closeModal();
            }
        }
    });

    function navigatePreview(direction) {
        if (currentFileIndex === -1) return;

        let newIndex = currentFileIndex + direction;
        if (newIndex < 0 || newIndex >= currentFiles.length) {
            return;
        }

        currentFileIndex = newIndex;
        let item = currentFiles[newIndex];
        previewFile(item);
    }

    $(window).on('popstate', function() {
        let newPath = window.location.pathname.replace(/^\/drive\//, '');
        if (newPath === 'drive' || newPath === '/drive') {
            newPath = '';
        }
        currentRelPath = newPath;
        loadDirectory(newPath);
    });

    function truncate(str, len) {
        if (!str) return '';
        if (str.length > len) {
            return str.substring(0, len-3) + '...';
        }
        return str;
    }

    function escapeHtml(str) {
        if (!str) return '';
        return str.replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }
});
