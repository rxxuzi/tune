package server

import (
	"github.com/rxxuzi/tune/internal/logger"
	"html/template"
	"io/ioutil"
	"net/http"
	"os/user"
	"path"
	"strconv"

	"github.com/google/uuid"
	"github.com/rxxuzi/tune/internal/static" // 埋め込みファイルシステム
)

// ハンドラの登録関数
func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/login/select", loginSelectHandler)
	mux.HandleFunc("/home", homeHandler)
	mux.HandleFunc("/terminal", terminalHandler)
	mux.HandleFunc("/terminal/ws", terminalWSHandler)
	mux.HandleFunc("/logout", logoutHandler)

	// テスト用ハンドラ
	mux.HandleFunc("/test", testTemplateHandler)

	// 静的ファイルのハンドラ
	webFS := http.FS(static.SubFS)
	fileServer := http.FileServer(webFS)
	mux.Handle("/web/", http.StripPrefix("/web/", fileServer))

	// ルートは/loginへリダイレクト
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	})
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

		logger.Info("SSH connection successful: %s@%s:%d", info.User, info.Host, info.Port)
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	// 保存済みホスト一覧を取得
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
	cfgFile := path.Join(u.HomeDir, ".tune", "verify", "ssh-"+targetHost+".json")
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

// テスト用ハンドラ
func testTemplateHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("/test accessed")
	err := template.Must(template.New("test").Parse(`
		{{ define "layout" }}
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<title>Test</title>
		</head>
		<body>
			{{ block "content" . }}{{ end }}
		</body>
		</html>
		{{ end }}
		{{ define "content" }}
		<h1>Test</h1>
		{{ end }}
	`)).ExecuteTemplate(w, "layout", nil)
	if err != nil {
		logger.Err("Template test error: %v", err)
		http.Error(w, "Template test error", http.StatusInternalServerError)
	}
}