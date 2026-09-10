package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"landwalker/internal/game"
	"landwalker/internal/model"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testServer(t *testing.T) http.Handler {
	t.Helper()
	games := game.NewGameManagerWithConfig(game.Config{RandomSeed: 1})
	t.Cleanup(games.Close)
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<h1>LandWalker</h1>")},
		"static/app.js": {Data: []byte("console.log('ok')")},
	}
	root, err := fs.Sub(assets, ".")
	if err != nil {
		t.Fatalf("fs.Sub() error = %v", err)
	}
	return New(games, root)
}

func TestHealth(t *testing.T) {
	response := httptest.NewRecorder()
	testServer(t).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", response.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("health response is not JSON: %v", err)
	}
	if body["engine"] != "Go" {
		t.Fatalf("health engine = %q, want Go", body["engine"])
	}
}

func TestGameAPIFlow(t *testing.T) {
	handler := testServer(t)
	aggression := 45
	payload, _ := json.Marshal(model.NewGameRequest{
		Rows: 10, Columns: 12, Difficulty: "easy", TotalLevels: 4, EnemyAggression: &aggression,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/game/new", bytes.NewReader(payload)))
	if response.Code != http.StatusCreated {
		t.Fatalf("new game status/body = %d/%s, want 201", response.Code, response.Body.String())
	}

	var created model.GameState
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("new game response is not JSON: %v", err)
	}
	if created.EnemyAggression != aggression {
		t.Fatalf("aggression = %d, want %d", created.EnemyAggression, aggression)
	}
	if created.Level != 1 || created.TotalLevels != 4 {
		t.Fatalf("floor = %d/%d, want 1/4", created.Level, created.TotalLevels)
	}

	stateResponse := httptest.NewRecorder()
	url := "/api/game/state?sessionId=" + created.SessionID
	handler.ServeHTTP(stateResponse, httptest.NewRequest(http.MethodGet, url, nil))
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("state status/body = %d/%s, want 200", stateResponse.Code, stateResponse.Body.String())
	}
}

func TestInvalidNewGameReturnsBadRequest(t *testing.T) {
	response := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"rows":2,"columns":12}`)
	testServer(t).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/game/new", body))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid game status = %d, want 400", response.Code)
	}
}

func TestIndexAndNotFound(t *testing.T) {
	handler := testServer(t)
	index := httptest.NewRecorder()
	handler.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if index.Code != http.StatusOK || index.Body.String() != "<h1>LandWalker</h1>" {
		t.Fatalf("index status/body = %d/%q", index.Code, index.Body.String())
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing path status = %d, want 404", missing.Code)
	}
}
