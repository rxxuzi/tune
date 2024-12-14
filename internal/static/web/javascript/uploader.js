document.addEventListener("DOMContentLoaded", () => {
    const treeContainer = document.getElementById("folderTree");
    const selectedFolderSpan = document.getElementById("selectedFolder");
    const destinationInput = document.getElementById("destination");
    const uploadButton = document.querySelector(".submit-button");
    const searchInput = document.getElementById("searchInput");
    const progressContainer = document.getElementById("progressContainer");
    const uploadProgress = document.getElementById("uploadProgress");
    const progressText = document.querySelector(".progress-text");
    const uploadStatus = document.getElementById("uploadStatus");
    const fileModal = document.getElementById("fileModal");
    const previewModal = document.getElementById("previewModal");
    const closeButtons = document.querySelectorAll(".close-button");
    const fileTableBody = document.querySelector("#fileTable tbody");
    const startUploadButton = document.getElementById("startUpload");
    const fileInput = document.getElementById("fileInput");
    const uploadForm = document.getElementById("uploadForm");
    const filePreviewContent = document.getElementById("filePreviewContent");
    const fileInputWrapper = document.querySelector(".file-input-wrapper");

    // フォルダツリーのデータを保持する変数
    let folderData = [];
    let selectedFiles = [];

    closeButtons.forEach(button => {
        button.addEventListener("click", () => {
            button.closest(".modal").style.display = "none";
        });
    });

    // モーダル外クリックで閉じる
    window.addEventListener("click", (event) => {
        if (event.target == fileModal) {
            fileModal.style.display = "none";
        }
        if (event.target == previewModal) {
            previewModal.style.display = "none";
        }
    });

    // フォルダツリーのデータをサーバーから取得
    fetch("/api/folder-tree")
        .then((response) => response.json())
        .then((data) => {
            folderData = sortFolders(data);
            buildTree(treeContainer, folderData, true); // 初期表示を折りたたんだ状態にする
        })
        .catch((error) => {
            console.error("Error fetching folder tree:", error);
        });

    // フォルダツリーをA-Zでソートする関数
    function sortFolders(nodes) {
        return nodes.sort((a, b) => a.name.localeCompare(b.name, 'ja'));
    }

    // フォルダツリーを構築
    function buildTree(parent, nodes, initiallyCollapsed = false) {
        nodes.forEach((node) => {
            const li = document.createElement("li");
            li.textContent = node.name;
            li.dataset.path = node.path;

            if (node.children && node.children.length > 0) {
                li.classList.add(initiallyCollapsed ? "collapsed" : "expanded");
                const ul = document.createElement("ul");
                ul.style.display = initiallyCollapsed ? "none" : "block";
                const sortedChildren = sortFolders(node.children);
                buildTree(ul, sortedChildren, initiallyCollapsed);
                li.appendChild(ul);

                // 折り畳みクリックイベント
                li.addEventListener("click", (e) => {
                    e.stopPropagation();
                    toggleFolder(li, node.path);
                });
            }

            // フォルダ選択イベント
            li.addEventListener("dblclick", (e) => {
                e.stopPropagation();
                selectFolder(node.path);
            });

            li.addEventListener("click", (e) => {
                e.stopPropagation();
                // シングルクリックでフォルダを選択
                selectFolder(node.path);
            });

            parent.appendChild(li);
        });
    }

    // フォルダを選択する関数
    function selectFolder(folderPath) {
        // スラッシュを統一
        const normalizedPath = folderPath.replace(/\\/g, "/");
        // 相対パスなので、~/ を付与
        const displayPath = normalizedPath === "" ? "~" : `~/${normalizedPath}`;
        selectedFolderSpan.textContent = displayPath;
        destinationInput.value = normalizedPath;
        uploadButton.disabled = false;
    }

    // フォルダツリーのスタイルを更新する関数
    function toggleFolder(li, path) {
        const ul = li.querySelector('ul');
        if (!ul) return;

        if (ul.style.display === "none") {
            // サブフォルダを取得して表示（遅延読み込み）
            if (ul.childElementCount === 0) {
                fetch(`/api/folder-tree?path=${encodeURIComponent(path)}`)
                    .then(response => response.json())
                    .then((data) => {
                        const sortedData = sortFolders(data);
                        buildTree(ul, sortedData, false);
                        ul.style.display = "block";
                        li.classList.remove("collapsed");
                        li.classList.add("expanded");
                    })
                    .catch((error) => {
                        console.error("Error fetching sub-folder tree:", error);
                    });
            } else {
                ul.style.display = "block";
                li.classList.remove("collapsed");
                li.classList.add("expanded");
            }
        } else {
            ul.style.display = "none";
            li.classList.remove("expanded");
            li.classList.add("collapsed");
        }
    }

    // フォルダツリーの検索機能
    searchInput.addEventListener("input", () => {
        const query = searchInput.value.toLowerCase();
        treeContainer.innerHTML = "";
        const filteredData = filterFolders(folderData, query);
        buildTree(treeContainer, filteredData, true);
    });

    // フォルダツリーをフィルタリングする関数
    function filterFolders(nodes, query) {
        return nodes
            .filter(node => node.name.toLowerCase().includes(query))
            .map(node => {
                const filteredNode = { ...node };
                if (node.children && node.children.length > 0) {
                    filteredNode.children = filterFolders(node.children, query);
                }
                return filteredNode;
            });
    }

    // ファイル入力の変更イベント
    fileInput.addEventListener("change", () => {
        const files = Array.from(fileInput.files);
        if (files.length > 0 && destinationInput.value !== "") {
            addSelectedFiles(files);
            populateFileModal(selectedFiles);
            fileModal.style.display = "block";
        } else if (files.length > 0 && destinationInput.value === "") {
            alert("Please select a destination folder first.");
            fileInput.value = ""; // ファイル選択をリセット
        }
    });

    // 選択されたファイルを追加する関数（重複を防ぐ）
    function addSelectedFiles(files) {
        files.forEach(file => {
            if (!selectedFiles.some(f => f.name === file.name && f.size === file.size && f.lastModified === file.lastModified)) {
                selectedFiles.push(file);
            }
        });
    }

    // ファイルモーダルをポピュレートする関数
    function populateFileModal(files) {
        fileTableBody.innerHTML = "";
        files.forEach((file, index) => {
            const tr = document.createElement("tr");

            // ファイル名
            const nameTd = document.createElement("td");
            nameTd.textContent = file.name;
            nameTd.classList.add("filename"); // 省略表示用クラス
            tr.appendChild(nameTd);

            // ファイルサイズ
            const sizeTd = document.createElement("td");
            sizeTd.textContent = formatBytes(file.size);
            tr.appendChild(sizeTd);

            // ファイルタイプ
            const typeTd = document.createElement("td");
            typeTd.textContent = file.type || "N/A";
            tr.appendChild(typeTd);

            // アクション
            const actionTd = document.createElement("td");

            // 削除ボタン
            const deleteButton = document.createElement("button");
            deleteButton.textContent = "Remove";
            deleteButton.classList.add("preview-button");
            deleteButton.addEventListener("click", () => {
                selectedFiles.splice(index, 1);
                populateFileModal(selectedFiles); // 再描画
            });
            actionTd.appendChild(deleteButton);

            // テキストファイルや画像の場合、プレビューボタンを追加（テキストのみ）
            if (isPreviewable(file)) {
                const previewButton = document.createElement("button");
                previewButton.textContent = "Preview"; // テキストのみ
                previewButton.classList.add("preview-button");
                previewButton.addEventListener("click", () => {
                    previewFile(file);
                });
                actionTd.appendChild(previewButton);
            }

            tr.appendChild(actionTd);
            fileTableBody.appendChild(tr);
        });

        // アップロードボタンの有効化
        if (selectedFiles.length > 0) {
            startUploadButton.disabled = false;
        } else {
            startUploadButton.disabled = true;
        }
    }

    // プレビューボタンが必要か判定する関数
    function isPreviewable(file) {
        const imageTypes = ['image/png', 'image/jpeg', 'image/gif', 'image/bmp', 'image/svg+xml'];
        const textTypes = ['text/plain', 'text/markdown', 'text/html', 'application/json'];
        return imageTypes.includes(file.type) || textTypes.includes(file.type) || /\.(md|html|json)$/i.test(file.name);
    }

    // ファイルのプレビュー関数
    function previewFile(file) {
        // Clear previous content
        filePreviewContent.innerHTML = "";

        if (isImageFile(file)) {
            // 画像ファイルの場合
            const img = document.createElement("img");
            img.style.maxWidth = "100%";
            img.style.maxHeight = "400px";
            const reader = new FileReader();
            reader.onload = function(e) {
                img.src = e.target.result;
                filePreviewContent.appendChild(img);
                previewModal.style.display = "block";
            };
            reader.onerror = function() {
                filePreviewContent.textContent = "Error loading image.";
                previewModal.style.display = "block";
            };
            reader.readAsDataURL(file);
        } else if (isTextFile(file)) {
            // テキストファイルの場合
            const pre = document.createElement("pre");
            pre.style.whiteSpace = "pre-wrap";
            pre.style.wordWrap = "break-word";
            pre.textContent = "Loading...";
            filePreviewContent.appendChild(pre);

            const reader = new FileReader();
            reader.onload = function(e) {
                pre.textContent = e.target.result;
                previewModal.style.display = "block";
            };
            reader.onerror = function() {
                pre.textContent = "Error loading file.";
                previewModal.style.display = "block";
            };
            reader.readAsText(file);
        } else {
            // その他のファイルタイプ
            filePreviewContent.textContent = "Preview not available for this file type.";
            previewModal.style.display = "block";
        }
    }

    function isImageFile(file) {
        const imageTypes = ['image/png', 'image/jpeg', 'image/gif', 'image/bmp', 'image/svg+xml'];
        return imageTypes.includes(file.type);
    }

    function isTextFile(file) {
        const textTypes = ['text/plain', 'text/markdown', 'text/html', 'application/json'];
        return textTypes.includes(file.type) || /\.(md|html|json)$/i.test(file.name);
    }

    function formatBytes(bytes, decimals = 2) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const dm = decimals < 0 ? 0 : decimals;
        const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
    }

    startUploadButton.addEventListener("click", () => {
        fileModal.style.display = "none";
        uploadSelectedFiles();
    });

    // 選択されたファイルをアップロードする関数
    function uploadSelectedFiles() {
        if (selectedFiles.length === 0) {
            alert("No files to upload.");
            return;
        }

        let formData = new FormData();
        selectedFiles.forEach(file => {
            formData.append('files', file);
        });
        formData.append('destination', destinationInput.value);

        // アップロード開始時に選択ファイルを表示
        uploadStatus.innerHTML = `
            <p>Uploading the following files:</p>
            <ul>
                ${selectedFiles.map(file => `<li>${file.name}</li>`).join('')}
            </ul>
        `;
        progressContainer.style.display = "block";
        uploadProgress.style.width = '0%';
        uploadProgress.textContent = '0%';
        progressText.textContent = '0%';

        $.ajax({
            xhr: function() {
                const xhr = new window.XMLHttpRequest();
                xhr.upload.addEventListener("progress", function(evt) {
                    if (evt.lengthComputable) {
                        const percentComplete = Math.round((evt.loaded / evt.total) * 100);
                        uploadProgress.style.width = percentComplete + '%';
                        uploadProgress.textContent = percentComplete + '%';
                        progressText.textContent = percentComplete + '%';
                    }
                }, false);
                return xhr;
            },
            url: "/api/upload",
            method: "POST",
            data: formData,
            processData: false,
            contentType: false,
            beforeSend: function() {
                uploadStatus.innerHTML += `<p>Uploading...</p>`;
                uploadButton.disabled = true;
            },
            success: function(response) {
                uploadStatus.innerHTML += `<p style="color: green;">Upload successful!</p>`;
                progressContainer.style.display = "none";
                uploadButton.disabled = false;
                // フォームをリセット
                uploadForm.reset();
                // 選択フォルダのリセット
                selectedFolderSpan.textContent = "None selected";
                destinationInput.value = "";
                // ファイルリストのリセット
                selectedFiles = [];
                fileTableBody.innerHTML = "";
            },
            error: function(err) {
                console.error("Upload failed:", err);
                uploadStatus.innerHTML += `<p style="color: red;">Upload failed. Please try again.</p>`;
                progressContainer.style.display = "none";
                uploadButton.disabled = false;
            }
        });
    }

    // フォームの送信イベントを無効化（モーダルで管理するため）
    uploadForm.addEventListener("submit", (e) => {
        e.preventDefault();
        const files = Array.from(fileInput.files);
        if (files.length > 0 && destinationInput.value !== "") {
            addSelectedFiles(files);
            populateFileModal(selectedFiles);
            fileModal.style.display = "block";
        } else if (files.length > 0 && destinationInput.value === "") {
            alert("Please select a destination folder first.");
            fileInput.value = ""; // ファイル選択をリセット
        }
    });

    // ドラッグ＆ドロップイベントの追加
    fileInputWrapper.addEventListener("dragover", (e) => {
        e.preventDefault();
        fileInputWrapper.classList.add("dragover");
    });

    fileInputWrapper.addEventListener("dragleave", () => {
        fileInputWrapper.classList.remove("dragover");
    });

    fileInputWrapper.addEventListener("drop", (e) => {
        e.preventDefault();
        fileInputWrapper.classList.remove("dragover");
        const files = Array.from(e.dataTransfer.files);
        if (files.length > 0 && destinationInput.value !== "") {
            addSelectedFiles(files);
            populateFileModal(selectedFiles);
            fileModal.style.display = "block";
        } else if (files.length > 0 && destinationInput.value === "") {
            alert("Please select a destination folder first.");
        }
    });
});
