package main

import (
	"github.com/rxxuzi/tune/internal/logger"
	"net/http"
	"os"

	"github.com/rxxuzi/tune/internal/server"
)

func main() {
	// 環境設定
	port := os.Getenv("TUNE_PORT")
	if port == "" {
		port = "9000"
	}

	// ハンドラ登録
	mux := http.NewServeMux()
	server.RegisterHandlers(mux)

	// サーバー起動
	addr := ":" + port
	logger.Info("Listening on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Fatal("Server startup failure: %v", err)
	}
}
