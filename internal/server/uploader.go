// uploader.go
package server

import (
	"encoding/json"
	"fmt"
	"github.com/rxxuzi/tune/internal/command"
	"io"
	"net/http"
	"path"

	"path/filepath"
	"strings"

	"github.com/rxxuzi/tune/internal/logger"
	"golang.org/x/crypto/ssh"
)

type FolderItem struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Type     string       `json:"type"` // "folder"
	Children []FolderItem `json:"children,omitempty"`
}

func RegisterUploaderHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/uploader", uploaderPageHandler)
	mux.HandleFunc("/api/folder-tree", folderTreeHandler)
	mux.HandleFunc("/api/upload", uploadHandler)
}

func uploaderPageHandler(w http.ResponseWriter, r *http.Request) {
	sess, err := getSession(r)
	if err != nil {
		logger.Err("Failed to retrieve session: %v", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		logger.Warn("Session does not contain session_id")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	_, exists := sshManager.GetClient(sessionID)
	if !exists {
		logger.Warn("SSH connection does not exist for session")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// upload.html を描画
	renderTemplate(w, "upload", nil)
}

func folderTreeHandler(w http.ResponseWriter, r *http.Request) {
	sess, err := getSession(r)
	if err != nil {
		logger.Err("Failed to retrieve session: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		logger.Warn("Session does not contain session_id")
		http.Error(w, "Invalid session", http.StatusBadRequest)
		return
	}

	client, exists := sshManager.GetClient(sessionID)
	if !exists || client == nil {
		logger.Warn("SSH connection does not exist for session")
		http.Error(w, "SSH not connected", http.StatusForbidden)
		return
	}

	tree, err := getRemoteFolderTree(client)
	if err != nil {
		logger.Err("Failed to get remote folder tree: %v", err)
		http.Error(w, "Failed to get folder tree", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess, err := getSession(r)
	if err != nil {
		logger.Err("Failed to retrieve session: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		logger.Warn("Session does not contain session_id")
		http.Error(w, "Invalid session", http.StatusBadRequest)
		return
	}

	client, exists := sshManager.GetClient(sessionID)
	if !exists || client == nil {
		logger.Warn("SSH connection does not exist for session")
		http.Error(w, "SSH not connected", http.StatusForbidden)
		return
	}

	err = r.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		logger.Err("Failed to parse multipart form: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	destination := r.FormValue("destination")
	if destination == "" {
		logger.Err("Destination path is missing")
		http.Error(w, "Destination path is required", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		logger.Err("No files uploaded")
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	// ホームディレクトリを取得
	homeDir, err := command.ExecuteCommand(client, "echo $HOME")
	if err != nil {
		logger.Err("Failed to get home directory: %v", err)
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}
	homeDir = strings.TrimSpace(homeDir) // 改行を削除

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			logger.Err("Failed to open uploaded file: %v", err)
			continue
		}

		// ファイルをアップロード後にクローズ
		// defer file.Close() はループ内で使用すると全てのファイルが最後に閉じられるため避ける
		remotePath := path.Join(destination, fileHeader.Filename)
		fullRemotePath := path.Join(homeDir, remotePath)

		// ログにリモートパスを表示
		logger.Debug("Uploading file to remote path: %s", fullRemotePath)

		err = uploadFile(client, file, fullRemotePath)
		if err != nil {
			logger.Err("Failed to upload file %s: %v", fullRemotePath, err)
			file.Close()
			continue
		}
		logger.Info("File uploaded successfully: %s", fullRemotePath)
		file.Close()
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Files uploaded successfully"))
}

// uploadFile ファイルをリモートホストにアップロードする
func uploadFile(client *ssh.Client, file io.Reader, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// scp コマンドを使用してファイルをアップロード
	// 'C0644 filesize filename' を送信し、続けてファイルデータを送信する
	cmd := fmt.Sprintf("scp -t %s", remotePath)
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	if err := session.Start(cmd); err != nil {
		return err
	}

	// ファイルの内容を読み込み
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	filesize := len(fileBytes)
	filename := filepath.Base(remotePath)

	// SCPプロトコルに従ってコマンドを送信
	fmt.Fprintf(stdin, "C0644 %d %s\n", filesize, filename)
	if _, err := stdin.Write(fileBytes); err != nil {
		return err
	}
	fmt.Fprint(stdin, "\x00") // 送信完了の通知
	stdin.Close()

	return session.Wait()
}

// getRemoteFolderTree はリモートサーバーのフォルダツリーを取得します
func getRemoteFolderTree(client *ssh.Client) ([]FolderItem, error) {
	// ユーザーのホームディレクトリを取得
	homeDir, err := command.ExecuteCommand(client, "echo $HOME")
	if err != nil {
		return nil, err
	}
	homeDir = strings.TrimSpace(homeDir) // 改行を削除
	logger.Debug("Home -> %s", homeDir)

	// フォルダツリーを取得
	cmdFind := fmt.Sprintf("find %s -type d -print 2>/dev/null", homeDir)

	output, err := command.ExecuteCommand(client, cmdFind)
	if err != nil {
		logger.Err("Failed to get remote folder tree: %v", err)
		return nil, err
	}

	dirs := strings.Split(output, "\n")

	// ホームディレクトリ基点の相対パスに変換
	var relativeDirs []string
	for _, dir := range dirs {
		if strings.HasPrefix(dir, homeDir) {
			trimmedDir := strings.TrimPrefix(dir, homeDir)
			trimmedDir = strings.TrimPrefix(trimmedDir, "/") // 最初のスラッシュを削除
			relativeDirs = append(relativeDirs, trimmedDir)
		}
	}

	tree := buildTreeFromPaths(relativeDirs)
	return tree, nil
}

func buildTreeFromPaths(paths []string) []FolderItem {
	root := FolderItem{
		Name:     "/",
		Path:     "/",
		Type:     "folder",
		Children: []FolderItem{},
	}

	for _, dir := range paths {
		if dir == "" {
			continue
		}
		parts := strings.Split(dir, "/")
		current := &root
		currentPath := ""
		for _, part := range parts {
			if part == "" {
				continue
			}
			currentPath = path.Join(currentPath, part)
			var child *FolderItem
			for i := range current.Children {
				if current.Children[i].Name == part {
					child = &current.Children[i]
					break
				}
			}
			if child == nil {
				newFolder := FolderItem{
					Name:     part,
					Path:     currentPath,
					Type:     "folder",
					Children: []FolderItem{},
				}
				current.Children = append(current.Children, newFolder)
				child = &current.Children[len(current.Children)-1]
			}
			current = child
		}
	}

	return root.Children
}
