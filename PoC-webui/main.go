package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

//go:embed frontend/dist/*
var embededFiles embed.FS

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(embededFiles, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	return http.FS(fsys)
}

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Serve static files from embedded FS
	assetHandler := http.FileServer(getFileSystem())
	e.GET("/*", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if file exists in the embedded FS
		path := r.URL.Path
		if path == "/" {
			assetHandler.ServeHTTP(w, r)
			return
		}

		// Clean path for fs.Stat
		cleanPath := path[1:]
		_, err := fs.Stat(embededFiles, "frontend/dist/"+cleanPath)
		if err != nil {
			// If file doesn't exist, serve index.html (for SPA routing)
			r.URL.Path = "/"
		}
		assetHandler.ServeHTTP(w, r)
	})))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on http://localhost:%s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
