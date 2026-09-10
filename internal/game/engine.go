package game

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"landwalker/internal/mapgen"
	"landwalker/internal/model"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"
)

const (
	defaultEnemyAggression = 70
	defaultEnemyMoveDelay  = 1500 * time.Millisecond
	defaultTotalLevels     = 3
	maxTotalLevels         = 10
)

type Config struct {
	EnemyMoveInterval time.Duration
	RandomSeed        int64
}

type GameManager struct {
	mu       sync.RWMutex
	sessions map[string]*model.GameState
	rng      *mathrand.Rand
	done     chan struct{}
	close    sync.Once
}

func NewGameManager() *GameManager {
	return NewGameManagerWithConfig(Config{EnemyMoveInterval: defaultEnemyMoveDelay})
}

func NewGameManagerWithConfig(config Config) *GameManager {
	seed := config.RandomSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	manager := &GameManager{
		sessions: make(map[string]*model.GameState),
		rng:      mathrand.New(mathrand.NewSource(seed)),
		done:     make(chan struct{}),
	}
	if config.EnemyMoveInterval > 0 {
		go manager.runEnemyLoop(config.EnemyMoveInterval)
	}
	return manager
}

func (manager *GameManager) Close() {
	manager.close.Do(func() { close(manager.done) })
}

func (manager *GameManager) NewGame(request model.NewGameRequest) (*model.GameState, error) {
	aggression := defaultEnemyAggression
	if request.EnemyAggression != nil {
		aggression = *request.EnemyAggression
	}
	if aggression < 0 || aggression > 100 {
		return nil, fmt.Errorf("enemy aggression must be between 0 and 100")
	}
	totalLevels := request.TotalLevels
	if totalLevels == 0 {
		totalLevels = defaultTotalLevels
	}
	if totalLevels < 1 || totalLevels > maxTotalLevels {
		return nil, fmt.Errorf("total levels must be between 1 and %d", maxTotalLevels)
	}

	generatedMap, err := mapgen.Generate(model.MapRequest{
		Rows:          request.Rows,
		Columns:       request.Columns,
		Seed:          request.Seed,
		Difficulty:    request.Difficulty,
		ElementCounts: request.ElementCounts,
	})
	if err != nil {
		return nil, err
	}

	difficulty := strings.ToLower(strings.TrimSpace(request.Difficulty))
	if difficulty == "" {
		difficulty = mapgen.DefaultDifficulty
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	playerRow, playerColumn := generatedMap.StartPosition[0], generatedMap.StartPosition[1]
	bearRow, bearColumn := spawnBear(generatedMap.Grid, playerRow, playerColumn, generatedMap.DoorPosition, manager.rng)
	gazelleRow, gazelleColumn := spawnGazelle(generatedMap.Grid, playerRow, playerColumn, bearRow, bearColumn, manager.rng)
	state := &model.GameState{
		SessionID:       generateSessionID(),
		Revision:        1,
		Level:           1,
		TotalLevels:     totalLevels,
		Rows:            generatedMap.Rows,
		Columns:         generatedMap.Columns,
		Seed:            generatedMap.Seed,
		Difficulty:      difficulty,
		EnemyAggression: aggression,
		ElementCounts:   cloneCounts(request.ElementCounts),
		Grid:            generatedMap.Grid,
		Player: model.Entity{
			ID: "player", Type: "player", Row: playerRow, Column: playerColumn,
			HP: 3, MaxHP: 3, Alive: true,
		},
		Entities: []model.Entity{
			{
				ID: "bear-1", Type: "bear", Mode: "patrolling", Row: bearRow, Column: bearColumn,
				HP: 5, MaxHP: 5, Alive: true, Hostile: true,
			},
			{
				ID: "gazelle-1", Type: "gazelle", Mode: "wandering", Row: gazelleRow, Column: gazelleColumn,
				HP: 1, MaxHP: 1, Alive: true,
			},
		},
		Message:    fmt.Sprintf("Floor 1/%d: find the door ⊕. Avoid the bear 🐻 and watch the gazelle 🦌.", totalLevels),
		Door:       append([]int(nil), generatedMap.DoorPosition...),
		Status:     "playing",
		EngineType: "Go",
	}
	manager.sessions[state.SessionID] = state
	return cloneState(state), nil
}

func (manager *GameManager) GetState(sessionID string) (*model.GameState, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	state, exists := manager.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	return cloneState(state), nil
}

func (manager *GameManager) Move(sessionID, direction string) (*model.GameState, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	state, exists := manager.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	if state.GameOver {
		return cloneState(state), nil
	}

	deltaRow, deltaColumn, ok := directionDelta(direction)
	if !ok {
		return nil, fmt.Errorf("invalid direction")
	}

	state.Revision++
	result := moveEntity(state, &state.Player, deltaRow, deltaColumn)
	if result.Moved {
		state.MovesCount++
	}
	state.Message = playerMovementMessage(result)

	if !state.Player.Alive {
		finishPlayerDeath(state, state.Message)
		return cloneState(state), nil
	}
	if bearCaughtPlayer(state) {
		return cloneState(state), nil
	}
	if state.Player.Row == state.Door[0] && state.Player.Column == state.Door[1] {
		if state.Level == state.TotalLevels {
			state.GameWon = true
			state.GameOver = true
			state.Status = "won"
			state.Message = fmt.Sprintf("🎉 Expedition complete! You escaped all %d floors in %d moves!", state.TotalLevels, state.MovesCount)
		} else if err := manager.advanceLevel(state); err != nil {
			return nil, err
		}
	}
	return cloneState(state), nil
}

func (manager *GameManager) advanceLevel(state *model.GameState) error {
	nextSeed := state.Seed + 1
	var generatedMap *model.MapResponse
	var err error
	for attempts := 0; attempts < 16; attempts++ {
		seed := nextSeed + int64(attempts)
		generatedMap, err = mapgen.Generate(model.MapRequest{
			Rows:          state.Rows,
			Columns:       state.Columns,
			Seed:          &seed,
			Difficulty:    state.Difficulty,
			ElementCounts: state.ElementCounts,
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("could not generate the next floor: %w", err)
	}

	playerRow, playerColumn := generatedMap.StartPosition[0], generatedMap.StartPosition[1]
	state.Player.Row = playerRow
	state.Player.Column = playerColumn
	state.Player.Elevation = 0
	state.Player.SnowSteps = 0
	state.Player.SnowDirection = ""

	occupied := [][2]int{{playerRow, playerColumn}}
	entities := make([]model.Entity, 0, len(state.Entities))
	followingBears := 0
	for _, entity := range state.Entities {
		if entity.Type != "bear" || !entity.Alive {
			continue
		}
		row, column := spawnBearAvoiding(generatedMap.Grid, playerRow, playerColumn, generatedMap.DoorPosition, occupied, manager.rng)
		entity.Row = row
		entity.Column = column
		entity.Elevation = 0
		entity.SnowSteps = 0
		entity.SnowDirection = ""
		entity.Mode = "patrolling"
		entities = append(entities, entity)
		occupied = append(occupied, [2]int{row, column})
		followingBears++
	}

	gazelleRow, gazelleColumn := spawnGazelleAvoiding(generatedMap.Grid, playerRow, playerColumn, occupied, manager.rng)
	entities = append(entities, model.Entity{
		ID: "gazelle-1", Type: "gazelle", Mode: "wandering", Row: gazelleRow, Column: gazelleColumn,
		HP: 1, MaxHP: 1, Alive: true,
	})

	state.Level++
	state.Seed = generatedMap.Seed
	state.Grid = generatedMap.Grid
	state.Door = append(state.Door[:0], generatedMap.DoorPosition...)
	state.Entities = entities
	state.Revision++
	state.Message = fmt.Sprintf("🚪 Floor %d/%d entered. %d bear(s) followed you.", state.Level, state.TotalLevels, followingBears)
	return nil
}

func (manager *GameManager) runEnemyLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			manager.tickNPCs()
		case <-manager.done:
			return
		}
	}
}

func (manager *GameManager) tickNPCs() {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for _, state := range manager.sessions {
		if state.GameOver {
			continue
		}
		manager.moveGazelles(state)
		resolvePredation(state)
		manager.moveBears(state)
		resolvePredation(state)
	}
}

func finishPlayerDeath(state *model.GameState, message string) {
	state.Player.Alive = false
	state.GameOver = true
	state.Status = "dead"
	state.Message = message
}

func bearCaughtPlayer(state *model.GameState) bool {
	for _, entity := range state.Entities {
		if entity.Type == "bear" && entity.Alive && samePosition(entity, state.Player) {
			finishPlayerDeath(state, "🐻 The bear caught you! Game over.")
			return true
		}
	}
	return false
}

func directionDelta(direction string) (int, int, bool) {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "north", "up", "w":
		return -1, 0, true
	case "south", "down", "s":
		return 1, 0, true
	case "west", "left", "a":
		return 0, -1, true
	case "east", "right", "d":
		return 0, 1, true
	default:
		return 0, 0, false
	}
}

func samePosition(first, second model.Entity) bool {
	return first.Row == second.Row && first.Column == second.Column
}

func generateSessionID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func cloneCounts(counts map[string]int) map[string]int {
	if counts == nil {
		return nil
	}
	clone := make(map[string]int, len(counts))
	for key, value := range counts {
		clone[key] = value
	}
	return clone
}

func cloneState(state *model.GameState) *model.GameState {
	clone := *state
	clone.Door = append([]int(nil), state.Door...)
	clone.Entities = append([]model.Entity(nil), state.Entities...)
	clone.ElementCounts = cloneCounts(state.ElementCounts)
	clone.Grid = make([][]model.Tile, len(state.Grid))
	for row := range state.Grid {
		clone.Grid[row] = append([]model.Tile(nil), state.Grid[row]...)
	}
	return &clone
}
