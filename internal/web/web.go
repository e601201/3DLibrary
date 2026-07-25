// Package web はビルド済みフロントエンド(frontend/ の Vite ビルド成果物)を
// バイナリに埋め込む。static/ の中身は `cd frontend && npm run build` で生成される。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embedded embed.FS

// Static はフロントエンドのビルド成果物を返す。
func Static() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
