package main

import (
	"embed"
	"io/fs"
	"landwalker/internal/game"
	"landwalker/internal/server"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed static
var embeddedFiles embed.FS

func main() {
	assets, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	games := game.NewGameManager()
	defer games.Close()

	webServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(games, assets),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("LandWalker is running at http://localhost:%s", port)
	log.Fatal(webServer.ListenAndServe())
}
