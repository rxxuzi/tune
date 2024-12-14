package server

import (
	"fmt"
	"github.com/rxxuzi/tune/internal/logger"
	"html/template"
	"io/ioutil"
	"net/http"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/rxxuzi/tune/internal/static" // 埋め込みファイルシステム
)

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/login/select", loginSelectHandler)
	mux.HandleFunc("/home", homeHandler)
	mux.HandleFunc("/terminal", terminalHandler)
	mux.HandleFunc("/terminal/ws", terminalWSHandler)
	mux.HandleFunc("/logout", logoutHandler)

	RegisterUploaderHandlers(mux)
	// 静的ファイルのハンドラ
	webFS := http.FS(static.SubFS)
	fileServer := http.FileServer(webFS)
	mux.Handle("/web/", http.StripPrefix("/web/", fileServer))

	// ルートは状況に応じて /login または /home へリダイレクト
	mux.HandleFunc("/", rootRedirectHandler)
}

// ルートリダイレクトハンドラ
func rootRedirectHandler(w http.ResponseWriter, r *http.Request) {
	sess, err := getSession(r)
	if err != nil {
		logger.Err("Failed to retrieve session: %v", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		logger.Warn("Session does not contain session_id. Redirecting to /login")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	_, exists := sshManager.GetClient(sessionID)
	if !exists {
		logger.Warn("No SSH connection exists for session. Redirecting to /login")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	logger.Info("Active SSH connection found. Redirecting to /home")
	http.Redirect(w, r, "/home", http.StatusFound)
}

// テンプレートのレンダリング関数
func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, err := template.ParseFS(static.SubFS, name+".html")
	if err != nil {
		logger.Err("Failed to load template (%s): %v", name, err)
		http.Error(w, "Template loading error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		logger.Err("Failed to execute template (%s): %v", name, err)
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
}

// ログインハンドラ
func loginHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("/login accessed (Method: %s)", r.Method)
	if r.Method == http.MethodPost {
		// フォームデータの取得
		host := r.FormValue("host")
		user := r.FormValue("user")
		portStr := r.FormValue("port")
		pw := r.FormValue("password")
		saveConnection := r.FormValue("save_connection") // 新しく追加

		port, err := strconv.Atoi(portStr)
		if err != nil {
			logger.Err("Failed to convert port number: %v", err)
			http.Error(w, "Invalid port number", http.StatusBadRequest)
			return
		}
		if port == 0 {
			port = 22
		}

		info := SSHInfo{
			Host:     host,
			User:     user,
			Port:     port,
			Password: pw,
		}
		logger.Info("Attempting SSH connection: %s@%s:%d", info.User, info.Host, info.Port)
		client, err := connectSSH(&info)
		if err != nil {
			logger.Err("SSH connection failed: %v", err)
			http.Error(w, "SSH connection failed", http.StatusUnauthorized)
			return
		}

		// セッションの取得
		sess, err := getSession(r)
		if err != nil {
			logger.Err("Failed to retrieve session: %v", err)
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		// 一意のセッションIDを生成
		sessionID := uuid.New().String()

		// セッションにデータを保存
		sess.Values["session_id"] = sessionID
		sess.Values["user"] = info.User
		sess.Values["host"] = info.Host

		// SSHクライアントをSSHManagerに保存
		sshManager.AddClient(sessionID, client)

		// セッションを保存
		if err := sess.Save(r, w); err != nil {
			logger.Err("Failed to save session: %v", err)
			http.Error(w, "Session save error", http.StatusInternalServerError)
			return
		}

		// Save Connectionがチェックされている場合、接続情報を保存
		if saveConnection == "on" {
			err := saveSSHInfo(&info)
			if err != nil {
				logger.Err("Failed to save SSH connection info: %v", err)
				// ユーザーにフィードバックを提供する場合はここに追加
			} else {
				logger.Info("SSH connection info saved for host: %s", info.Host)
			}
		}

		logger.Info("SSH connection successful: %s@%s:%d", info.User, info.Host, info.Port)
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	// GETリクエストの場合の処理（既存のコード）
	hosts, err := loadSavedHosts()
	if err != nil {
		logger.Warn("Failed to load saved hosts: %v", err)
		hosts = []SSHInfo{}
	}

	// テンプレート用データを作成
	data := struct {
		Hosts []SSHInfo
	}{
		Hosts: hosts,
	}
	logger.Info("Displaying login page. Saved hosts count: %d", len(hosts))
	renderTemplate(w, "login", data)
}

// 保存済みホストからのログインハンドラ
func loginSelectHandler(w http.ResponseWriter, r *http.Request) {
	// URLパラメータからホストを取得
	targetHost := r.URL.Query().Get("host")
	if targetHost == "" {
		logger.Err("/login/select accessed without a host specified")
		http.Error(w, "Host not specified", http.StatusBadRequest)
		return
	}

	logger.Info("Attempting connection to saved host: %s", targetHost)
	u, err := user.Current()
	if err != nil {
		logger.Err("Failed to retrieve user info: %v", err)
		http.Error(w, "User info retrieval failed", http.StatusInternalServerError)
		return
	}
	cfgFile := filepath.Join(u.HomeDir, ".tune", "verify", fmt.Sprintf("ssh-%s.json", targetHost)) // filepath.Join とファイル名の形式変更
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		logger.Err("Failed to read host config file (%s): %v", cfgFile, err)
		http.Error(w, "Failed to read host config", http.StatusInternalServerError)
		return
	}

	// ホスト設定を解析
	info, err := parseJSONToSSHInfo(data)
	if err != nil {
		logger.Err("Failed to parse host config (%s): %v", cfgFile, err)
		http.Error(w, "Failed to parse host config", http.StatusInternalServerError)
		return
	}

	logger.Info("Attempting SSH connection: %s@%s:%d", info.User, info.Host, info.Port)
	client, err := connectSSH(&info)
	if err != nil {
		logger.Err("SSH connection failed: %v", err)
		http.Error(w, "SSH connection failed", http.StatusUnauthorized)
		return
	}

	// セッションの取得
	sess, err := getSession(r)
	if err != nil {
		logger.Err("Failed to retrieve session: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// セッションIDを生成し保存
	sessionID := uuid.New().String()
	sess.Values["session_id"] = sessionID
	sess.Values["user"] = info.User
	sess.Values["host"] = info.Host
	sshManager.AddClient(sessionID, client)

	if err := sess.Save(r, w); err != nil {
		logger.Err("Failed to save session: %v", err)
		http.Error(w, "Session save error", http.StatusInternalServerError)
		return
	}

	logger.Info("SSH connection successful: %s@%s:%d", info.User, info.Host, info.Port)
	http.Redirect(w, r, "/home", http.StatusFound)
}

// ホームハンドラ
func homeHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("/home accessed")
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
		logger.Warn("Session does not contain SSH information")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	user, ok := sess.Values["user"].(string)
	if !ok || user == "" {
		logger.Warn("Session does not contain user")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	host, ok := sess.Values["host"].(string)
	if !ok || host == "" {
		logger.Warn("Session does not contain host")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	data := struct {
		UserHost string
	}{
		UserHost: user + "@" + host,
	}
	logger.Info("Displaying home screen for: %s", data.UserHost)
	renderTemplate(w, "home", data)
}

// ターミナルハンドラ
func terminalHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("/terminal accessed")
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
		logger.Warn("Session does not contain SSH information")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	renderTemplate(w, "terminal", nil)
}

// ログアウトハンドラ
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("/logout accessed")
	sess, err := getSession(r)
	if err != nil {
		logger.Err("Failed to retrieve session: %v", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if ok && sessionID != "" {
		// SSHManagerからSSHクライアントを削除
		sshManager.RemoveClient(sessionID)
	}

	// セッションをクリア
	clearSession(w, r)
	logger.Info("Session cleared. Logging out.")
	http.Redirect(w, r, "/login", http.StatusFound)
}
