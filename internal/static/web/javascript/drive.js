$(document).ready(function() {
    const folderGrid = $('#folderGrid');
    const fileGrid = $('#fileGrid');
    const currentPath = $('#currentPath');
    const previewModal = $('#previewModal');
    const previewArea = $('#previewArea');
    const downloadLink = $('#downloadLink');
    const closeButton = $('.close-button');

    let currentRelPath = initialPath;

    // ローディングメッセージ表示用要素追加
    const loadingMessage = $('<div id="loadingMessage" style="text-align:center; color:var(--text-secondary); margin:1rem;">Now Loading...</div>');
    $('.drive-container').append(loadingMessage);

    function showLoading() {
        loadingMessage.show();
    }

    function hideLoading() {
        loadingMessage.hide();
    }

    loadDirectory(currentRelPath);

    function loadDirectory(p) {
        showLoading(); // ローディング表示
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

        // data.folders や data.files が存在しない場合、空配列を使用
        const folders = data.folders || [];
        const files = data.files || [];

        if (p === '') {
            currentPath.text('~/');
        } else {
            currentPath.text('~/' + p);
        }

        // 親ディレクトリリンクを追加（必要な場合）
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

        // フォルダの表示
        folders.forEach(item => {
            const div = $(`
                <div class="item folder">
                    <div class="icon-name">
                        <span class="material-icons">folder</span>
                        <span title="${escapeHtml(item.name)}">${truncate(item.name,20)}</span>
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

        // ファイルの表示
        files.forEach(item => {
            const div = $(`
                <div class="item file">
                    <div class="icon-name">
                        <span class="material-icons">insert_drive_file</span>
                        <span title="${escapeHtml(item.name)}">${truncate(item.name,20)}</span>
                    </div>
                </div>
            `);
            div.click(function(){
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
        showLoading();
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
        previewArea.html(contentHtml);
        downloadLink.html(`<a href="/api/drive/download?file=${encodeURIComponent(filePath)}" class="material-icons" style="font-size:24px; color:white;">download</a>`);
        previewModal.show();
    }

    function closeModal() {
        previewModal.hide();
        previewArea.html('');
        downloadLink.html('');
    }

    closeButton.click(function() {
        closeModal();
    });

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
