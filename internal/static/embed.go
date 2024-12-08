package static

import (
	"embed"
	"io/fs"
)

// webディレクトリ内の全てのHTML, CSS, JSファイルを埋め込む
//
//go:embed web
var embeddedFiles embed.FS

var SubFS fs.FS

func init() {
	var err error
	// "web"ディレクトリを基点としたサブファイルシステムを作成
	SubFS, err = fs.Sub(embeddedFiles, "web")
	if err != nil {
		panic(err)
	}
}
