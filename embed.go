package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// GetTemplatesFS 返回嵌入的模板文件系统
func GetTemplatesFS() embed.FS {
	return templatesFS
}

// GetStaticFS 返回嵌入的静态文件系统
func GetStaticFS() http.FileSystem {
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(staticSubFS)
}
