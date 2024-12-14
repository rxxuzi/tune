package server

import (
	"github.com/gorilla/websocket"
	"github.com/rxxuzi/tune/internal/logger"
	"golang.org/x/crypto/ssh"
	"io"
	"net/http"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// セキュリティ要件に応じてオリジンをチェック
		return true
	},
}

func terminalWSHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("/terminal/ws WebSocket connection request received")

	// セッションの取得
	sess, err := getSession(r)
	if err != nil {
		logger.Err("WebSocket: Failed to retrieve session: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		logger.Warn("WebSocket: session_id not found in session")
		http.Error(w, "SSH not connected", http.StatusForbidden)
		return
	}

	client, exists := sshManager.GetClient(sessionID)
	if !exists || client == nil {
		logger.Warn("WebSocket: SSH connection does not exist")
		http.Error(w, "SSH not connected", http.StatusForbidden)
		return
	}

	// WebSocket 接続のアップグレード
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Err("WebSocket: Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	logger.Info("WebSocket: Connection established")

	// SSH セッションの作成
	sshSession, err := client.NewSession()
	if err != nil {
		logger.Err("WebSocket: Failed to create SSH session: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to create SSH session\n"))
		return
	}
	defer sshSession.Close()

	// PTY のリクエスト（固定サイズ）
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // エコーを有効にする
		ssh.TTY_OP_ISPEED: 14400, // 入力速度
		ssh.TTY_OP_OSPEED: 14400, // 出力速度
	}
	cols := 32
	rows := 120
	if err := sshSession.RequestPty("xterm", cols, rows, modes); err != nil {
		logger.Err("WebSocket: PTY request failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("PTY request failed\n"))
		return
	}

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		logger.Err("WebSocket: StdinPipe failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to initialize StdinPipe\n"))
		return
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		logger.Err("WebSocket: StdoutPipe failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to initialize StdoutPipe\n"))
		return
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		logger.Err("WebSocket: StderrPipe failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to initialize StderrPipe\n"))
		return
	}

	if err := sshSession.Shell(); err != nil {
		logger.Err("WebSocket: Failed to start shell: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to start shell\n"))
		return
	}

	// SSH stdout を WebSocket に送信
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					logger.Err("WebSocket: Error reading from stdout: %v", err)
				}
				break
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil { // TextMessage -> BinaryMessage
				logger.Err("WebSocket: Error writing to WebSocket: %v", err)
				break
			}
		}
	}()

	// SSH stderr を WebSocket に送信
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err != io.EOF {
					logger.Err("WebSocket: Error reading from stderr: %v", err)
				}
				break
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil { // TextMessage -> BinaryMessage
				logger.Err("WebSocket: Error writing to WebSocket: %v", err)
				break
			}
		}
	}()

	// WebSocket メッセージを SSH stdin に送信
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Err("WebSocket: Unexpected client disconnection: %v", err)
			} else {
				logger.Info("WebSocket: Client disconnected: %v", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			// 通常の入力データとして扱う
			if _, err := stdin.Write(p); err != nil {
				logger.Err("WebSocket: Error writing to stdin: %v", err)
				break
			}

			// 'exit' コマンドの検出
			input := string(p)
			if input == "exit\n" || input == "exit\r\n" {
				logger.Info("WebSocket: 'exit' command received, ending session")
				// SSHManager からクライアントを削除
				sshManager.RemoveClient(sessionID)

				// クライアントに 'logout' メッセージを送信
				if err := conn.WriteMessage(websocket.TextMessage, []byte("logout")); err != nil {
					logger.Err("WebSocket: Failed to send 'logout' message: %v", err)
				}

				// SSH セッションを閉じる
				if err := sshSession.Close(); err != nil {
					logger.Err("WebSocket: Failed to close SSH session: %v", err)
				}

				// WebSocket を閉じる
				if err := conn.Close(); err != nil {
					logger.Err("WebSocket: Failed to close WebSocket: %v", err)
				}

				break
			}
		}
	}
	logger.Info("WebSocket: Session ended")
}
