package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/rxxuzi/tune/internal/command"
	"github.com/rxxuzi/tune/internal/logger"
	"github.com/rxxuzi/tune/internal/static"
)

type DriveItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type DriveTemplateData struct {
	UserHost string
	SubPath  string
}

func RegisterDriveHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/drive/", driveHandler)
	mux.HandleFunc("/api/drive/list", driveAPIHandler)
	mux.HandleFunc("/api/drive/preview", drivePreviewHandler)
	mux.HandleFunc("/api/drive/download", driveDownloadHandler)
}

func driveHandler(w http.ResponseWriter, r *http.Request) {
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

	client, exists := sshManager.GetClient(sessionID)
	if !exists || client == nil {
		logger.Warn("SSH connection does not exist for session")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	logger.Debug("/drive accessed")

	// サブパス解析: /drive/ の後ろをサブパスとして扱う
	subPath := strings.TrimPrefix(r.URL.Path, "/drive")
	subPath = strings.TrimPrefix(subPath, "/")

	data := DriveTemplateData{
		UserHost: getUserHost(sess),
		SubPath:  subPath,
	}

	renderDriveTemplate(w, "drive.html", data)
}

func getUserHost(sess *sessions.Session) string {
	user, ok1 := sess.Values["user"].(string)
	host, ok2 := sess.Values["host"].(string)
	if ok1 && ok2 {
		return fmt.Sprintf("%s@%s", user, host)
	}
	return "Unknown"
}

func renderDriveTemplate(w http.ResponseWriter, tmplName string, data DriveTemplateData) {
	tmpl, err := template.ParseFS(static.SubFS, tmplName)
	if err != nil {
		logger.Err("Failed to load template (%s): %v", tmplName, err)
		http.Error(w, "Template loading error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		logger.Err("Failed to execute template (%s): %v", tmplName, err)
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
	logger.Debug("Temple Render Done.")
}

func driveAPIHandler(w http.ResponseWriter, r *http.Request) {
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

	remotePath := r.URL.Query().Get("path")
	homeDir, err := command.ExecuteCommand(client, "echo $HOME")
	if err != nil {
		logger.Err("Failed to get home directory: %v", err)
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}
	homeDir = strings.TrimSpace(homeDir)

	if remotePath == "" {
		remotePath = homeDir
	} else {
		// 絶対パス化
		if !strings.HasPrefix(remotePath, homeDir) {
			remotePath = path.Join(homeDir, remotePath)
		}
	}

	// フォルダを取得
	cmdFolders := fmt.Sprintf("find '%s' -maxdepth 1 -mindepth 1 -type d -printf '%%f\\n'", remotePath)
	folderOutput, err := command.ExecuteCommand(client, cmdFolders)
	if err != nil {
		logger.Err("Failed to list folders (%s): %v", remotePath, err)
		http.Error(w, "Failed to list folders", http.StatusInternalServerError)
		return
	}

	// ファイルを取得
	cmdFiles := fmt.Sprintf("find '%s' -maxdepth 1 -mindepth 1 -type f -printf '%%f\\n'", remotePath)
	fileOutput, err := command.ExecuteCommand(client, cmdFiles)
	if err != nil {
		logger.Err("Failed to list files (%s): %v", remotePath, err)
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	var folders []DriveItem
	var files []DriveItem

	if strings.TrimSpace(folderOutput) != "" {
		for _, line := range strings.Split(strings.TrimSpace(folderOutput), "\n") {
			if line != "" {
				relPath := strings.TrimPrefix(remotePath, homeDir)
				relPath = strings.TrimPrefix(relPath, "/")
				folders = append(folders, DriveItem{
					Name: line,
					Path: path.Join(relPath, line),
					Type: "folder",
				})
			}
		}
	}

	if strings.TrimSpace(fileOutput) != "" {
		for _, line := range strings.Split(strings.TrimSpace(fileOutput), "\n") {
			if line != "" {
				relPath := strings.TrimPrefix(remotePath, homeDir)
				relPath = strings.TrimPrefix(relPath, "/")
				files = append(files, DriveItem{
					Name: line,
					Path: path.Join(relPath, line),
					Type: "file",
				})
			}
		}
	}

	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Name < folders[j].Name
	})

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	response := struct {
		Folders []DriveItem `json:"folders"`
		Files   []DriveItem `json:"files"`
	}{
		Folders: folders,
		Files:   files,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func drivePreviewHandler(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if file == "" {
		http.Error(w, "File not specified", http.StatusBadRequest)
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

	homeDir, err := command.ExecuteCommand(client, "echo $HOME")
	if err != nil {
		logger.Err("Failed to get home directory: %v", err)
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}
	homeDir = strings.TrimSpace(homeDir)
	absPath := path.Join(homeDir, file)

	// MIMEタイプ取得
	ftypeCmd := fmt.Sprintf("file -b --mime-type '%s'", absPath)
	mimeOut, err := command.ExecuteCommand(client, ftypeCmd)
	if err != nil {
		logger.Err("Failed to get mime type: %v", err)
		http.Error(w, "Failed to determine file type", http.StatusInternalServerError)
		return
	}

	mimeType := strings.TrimSpace(mimeOut)

	// JSONでMIMEタイプを返す
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"mime": mimeType,
	})
}

func driveDownloadHandler(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if file == "" {
		http.Error(w, "File not specified", http.StatusBadRequest)
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

	homeDir, err := command.ExecuteCommand(client, "echo $HOME")
	if err != nil {
		logger.Err("Failed to get home dir: %v", err)
		http.Error(w, "Failed home dir", http.StatusInternalServerError)
		return
	}
	homeDir = strings.TrimSpace(homeDir)
	absPath := path.Join(homeDir, file)

	cmd := fmt.Sprintf("cat '%s'", absPath)
	session, err := client.NewSession()
	if err != nil {
		logger.Err("Failed to create session for download: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		logger.Err("Failed to get stdout pipe: %v", err)
		http.Error(w, "Pipe error", http.StatusInternalServerError)
		return
	}

	if err := session.Start(cmd); err != nil {
		logger.Err("Failed to start cat command: %v", err)
		http.Error(w, "Failed to start file read", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(absPath)))
	w.Header().Set("Content-Type", "application/octet-stream")

	if _, err := io.Copy(w, stdout); err != nil {
		logger.Err("Failed to copy file data to response: %v", err)
		return
	}

	if err := session.Wait(); err != nil {
		logger.Err("Session wait error: %v", err)
		return
	}
}
