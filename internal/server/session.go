package server

import (
	"net/http"

	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore([]byte("secret-key"))

// セッション名
const sessionName = "tune-session"

// セッションからSSHクライアント情報やユーザ情報を管理
func getSession(r *http.Request) (*sessions.Session, error) {
	return store.Get(r, sessionName)
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	sess, _ := getSession(r)
	if sess != nil {
		sess.Options.MaxAge = -1
		sess.Save(r, w)
	}
}
