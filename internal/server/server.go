package server

import (
	"encoding/json"
	"io/fs"
	"landwalker/internal/game"
	"landwalker/internal/model"
	"net/http"
)

type Server struct {
	games  *game.GameManager
	assets fs.FS
}

func New(games *game.GameManager, assets fs.FS) http.Handler {
	server := &Server{games: games, assets: assets}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("/api/health", server.health)
	mux.HandleFunc("/api/game/new", server.newGame)
	mux.HandleFunc("/api/game/move", server.move)
	mux.HandleFunc("/api/game/state", server.state)
	mux.HandleFunc("/", server.index)
	return mux
}

func (server *Server) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{
		"status":  "UP",
		"engine":  "Go",
		"version": "2.0.0",
	})
}

func (server *Server) newGame(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var input model.NewGameRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid request body")
		return
	}
	state, err := server.games.NewGame(input)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, state)
}

func (server *Server) move(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	sessionID := request.URL.Query().Get("sessionId")
	if sessionID == "" {
		writeError(writer, http.StatusBadRequest, "missing sessionId parameter")
		return
	}
	var input model.MoveRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid move payload")
		return
	}
	state, err := server.games.Move(sessionID, input.Direction)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, state)
}

func (server *Server) state(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	sessionID := request.URL.Query().Get("sessionId")
	if sessionID == "" {
		writeError(writer, http.StatusBadRequest, "missing sessionId parameter")
		return
	}
	state, err := server.games.GetState(sessionID)
	if err != nil {
		writeError(writer, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, state)
}

func (server *Server) index(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	content, err := fs.ReadFile(server.assets, "index.html")
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "interface unavailable")
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write(content)
}

func decodeJSON(request *http.Request, value any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func methodNotAllowed(writer http.ResponseWriter, allowed string) {
	writer.Header().Set("Allow", allowed)
	writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
