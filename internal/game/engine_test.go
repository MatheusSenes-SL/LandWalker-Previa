package game

import (
	"landwalker/internal/model"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *GameManager {
	t.Helper()
	manager := NewGameManagerWithConfig(Config{RandomSeed: 1})
	t.Cleanup(manager.Close)
	return manager
}

func newTestGame(t *testing.T, manager *GameManager) *model.GameState {
	t.Helper()
	state, err := manager.NewGame(model.NewGameRequest{Rows: 10, Columns: 12, Difficulty: "normal"})
	if err != nil {
		t.Fatalf("NewGame() error = %v", err)
	}
	return state
}

func TestNewGameCreatesGoSession(t *testing.T) {
	manager := newTestManager(t)
	state := newTestGame(t, manager)

	if state.SessionID == "" {
		t.Fatal("NewGame() returned an empty session ID")
	}
	if state.Rows != 10 || state.Columns != 12 {
		t.Fatalf("NewGame() size = %dx%d, want 10x12", state.Rows, state.Columns)
	}
	if state.Player.HP != 3 || state.EngineType != "Go" {
		t.Fatalf("NewGame() player HP/engine = %d/%q, want 3/Go", state.Player.HP, state.EngineType)
	}
	if state.EnemyAggression != defaultEnemyAggression {
		t.Fatalf("NewGame() aggression = %d, want %d", state.EnemyAggression, defaultEnemyAggression)
	}
	if state.Level != 1 || state.TotalLevels != defaultTotalLevels {
		t.Fatalf("NewGame() floor = %d/%d, want 1/%d", state.Level, state.TotalLevels, defaultTotalLevels)
	}
	if len(state.Entities) != 2 || state.Entities[1].Type != "gazelle" {
		t.Fatalf("NewGame() entities = %+v, want bear and gazelle", state.Entities)
	}
}

func TestNewGameValidatesAggression(t *testing.T) {
	manager := newTestManager(t)
	invalid := 101
	_, err := manager.NewGame(model.NewGameRequest{EnemyAggression: &invalid})
	if err == nil {
		t.Fatal("NewGame() accepted aggression over 100")
	}
}

func TestNewGameValidatesTotalLevels(t *testing.T) {
	manager := newTestManager(t)
	_, err := manager.NewGame(model.NewGameRequest{TotalLevels: maxTotalLevels + 1})
	if err == nil {
		t.Fatal("NewGame() accepted too many levels")
	}
}

func TestDoorAdvancesFloorsAndPreservesLivingBear(t *testing.T) {
	manager := newTestManager(t)
	state, err := manager.NewGame(model.NewGameRequest{
		Rows: 10, Columns: 20, Difficulty: "normal", TotalLevels: 2,
	})
	if err != nil {
		t.Fatalf("NewGame() error = %v", err)
	}

	stored := manager.sessions[state.SessionID]
	stored.Entities[0].HP = 3
	bearID := stored.Entities[0].ID
	firstSeed := stored.Seed
	direction := prepareDoorEntry(stored)
	advanced := mustMove(t, manager, state.SessionID, direction)

	if advanced.Level != 2 || advanced.GameOver || advanced.GameWon {
		t.Fatalf("first door state = floor %d, over=%t, won=%t", advanced.Level, advanced.GameOver, advanced.GameWon)
	}
	if advanced.Seed == firstSeed {
		t.Fatal("entering a new floor did not generate a new room")
	}
	bear := findEntity(advanced.Entities, bearID)
	if bear == nil || !bear.Alive || bear.HP != 3 {
		t.Fatalf("following bear = %+v, want living bear with 3 HP", bear)
	}
	if advanced.Player.Row != advanced.Rows/2 || advanced.Player.Column != advanced.Columns/2 {
		t.Fatalf("player spawn = (%d,%d), want room center", advanced.Player.Row, advanced.Player.Column)
	}

	stored = manager.sessions[state.SessionID]
	direction = prepareDoorEntry(stored)
	won := mustMove(t, manager, state.SessionID, direction)
	if won.Level != 2 || !won.GameOver || !won.GameWon || won.Status != "won" {
		t.Fatalf("last door state = floor %d, over=%t, won=%t, status=%q", won.Level, won.GameOver, won.GameWon, won.Status)
	}
}

func TestSingleFloorWinsAtFirstDoor(t *testing.T) {
	manager := newTestManager(t)
	state, err := manager.NewGame(model.NewGameRequest{Rows: 8, Columns: 10, TotalLevels: 1})
	if err != nil {
		t.Fatalf("NewGame() error = %v", err)
	}
	direction := prepareDoorEntry(manager.sessions[state.SessionID])
	won := mustMove(t, manager, state.SessionID, direction)
	if !won.GameWon || won.Level != 1 {
		t.Fatalf("single-floor door did not win: %+v", won)
	}
}

func prepareDoorEntry(state *model.GameState) string {
	doorRow, doorColumn := state.Door[0], state.Door[1]
	direction := ""
	switch {
	case doorRow == 0:
		state.Player.Row, state.Player.Column, direction = 1, doorColumn, "north"
	case doorRow == state.Rows-1:
		state.Player.Row, state.Player.Column, direction = state.Rows-2, doorColumn, "south"
	case doorColumn == 0:
		state.Player.Row, state.Player.Column, direction = doorRow, 1, "west"
	default:
		state.Player.Row, state.Player.Column, direction = doorRow, state.Columns-2, "east"
	}
	state.Player.Elevation = 0
	state.Player.SnowSteps = 0
	state.Player.SnowDirection = ""
	occupied := map[[2]int]bool{{state.Player.Row, state.Player.Column}: true}
	for index := range state.Entities {
		placed := false
		for row := 1; row < state.Rows-1 && !placed; row++ {
			for column := 1; column < state.Columns-1; column++ {
				position := [2]int{row, column}
				if !occupied[position] {
					state.Entities[index].Row, state.Entities[index].Column = row, column
					occupied[position] = true
					placed = true
					break
				}
			}
		}
	}
	return direction
}

func findEntity(entities []model.Entity, id string) *model.Entity {
	for index := range entities {
		if entities[index].ID == id {
			return &entities[index]
		}
	}
	return nil
}

func TestMoveAndWallCollision(t *testing.T) {
	manager := newTestManager(t)
	state := newTestGame(t, manager)
	internal := manager.sessions[state.SessionID]
	row, column := internal.Player.Row, internal.Player.Column
	internal.Grid[row-1][column] = groundTile(row-1, column)

	moved, err := manager.Move(state.SessionID, "north")
	if err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if moved.Player.Row != row-1 || moved.MovesCount != 1 {
		t.Fatalf("Move() player row/moves = %d/%d, want %d/1", moved.Player.Row, moved.MovesCount, row-1)
	}

	internal.Grid[row-2][column] = model.Tile{Row: row - 2, Column: column, TypeName: "wall", DisplayElement: "#"}
	blocked, err := manager.Move(state.SessionID, "north")
	if err != nil {
		t.Fatalf("Move() into wall error = %v", err)
	}
	if blocked.Player.Row != row-1 || blocked.MovesCount != 1 {
		t.Fatalf("blocked Move() changed player to row %d with %d moves", blocked.Player.Row, blocked.MovesCount)
	}
}

func TestHoleCollapsesOnSecondVisit(t *testing.T) {
	manager := newTestManager(t)
	state := newTestGame(t, manager)
	internal := manager.sessions[state.SessionID]
	row, column := internal.Player.Row, internal.Player.Column
	internal.Grid[row-1][column] = model.Tile{
		Row: row - 1, Column: column, TypeID: "#TM901", TypeName: "hole",
		DisplayElement: "○", Walkable: true,
	}
	internal.Grid[row-1][column+1] = groundTile(row-1, column+1)

	mustMove(t, manager, state.SessionID, "north")
	mustMove(t, manager, state.SessionID, "east")
	result := mustMove(t, manager, state.SessionID, "west")

	if result.Grid[row-1][column].HoleVisits != 2 {
		t.Fatalf("hole visits = %d, want 2", result.Grid[row-1][column].HoleVisits)
	}
	if result.Player.HP != 2 {
		t.Fatalf("player HP = %d, want 2", result.Player.HP)
	}
}

func TestHighGroundRequiresRamp(t *testing.T) {
	manager := newTestManager(t)
	state := newTestGame(t, manager)
	internal := manager.sessions[state.SessionID]
	row, column := internal.Player.Row, internal.Player.Column
	internal.Grid[row-1][column] = model.Tile{
		Row: row - 1, Column: column, TypeName: "high",
		DisplayElement: "▲", Walkable: true, Elevation: 1,
	}

	blocked := mustMove(t, manager, state.SessionID, "north")
	if blocked.Player.Row != row {
		t.Fatalf("player climbed high ground without a ramp")
	}

	internal.Grid[row-1][column] = model.Tile{
		Row: row - 1, Column: column, TypeName: "ramp",
		DisplayElement: "↑", Walkable: true, RampDirection: "north",
	}
	climbed := mustMove(t, manager, state.SessionID, "north")
	if climbed.Player.Row != row-1 {
		t.Fatalf("player did not enter ramp: row = %d, want %d", climbed.Player.Row, row-1)
	}
}

func TestEnemyPursuesWithoutPlayerMove(t *testing.T) {
	aggression := 100
	manager := NewGameManagerWithConfig(Config{EnemyMoveInterval: 5 * time.Millisecond, RandomSeed: 1})
	t.Cleanup(manager.Close)
	state, err := manager.NewGame(model.NewGameRequest{
		Rows: 10, Columns: 12, Difficulty: "normal", EnemyAggression: &aggression,
	})
	if err != nil {
		t.Fatalf("NewGame() error = %v", err)
	}

	manager.mu.Lock()
	internal := manager.sessions[state.SessionID]
	playerRow, playerColumn := internal.Player.Row, internal.Player.Column
	internal.Entities[0].Row, internal.Entities[0].Column = playerRow, playerColumn-3
	internal.Entities[1].Row, internal.Entities[1].Column = 1, 1
	for column := playerColumn - 3; column <= playerColumn; column++ {
		internal.Grid[playerRow][column] = groundTile(playerRow, column)
	}
	startColumn := internal.Entities[0].Column
	manager.mu.Unlock()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		current, getErr := manager.GetState(state.SessionID)
		if getErr != nil {
			t.Fatalf("GetState() error = %v", getErr)
		}
		if current.Entities[0].Column > startColumn {
			if current.MovesCount != 0 {
				t.Fatalf("enemy movement changed player move count to %d", current.MovesCount)
			}
			if current.Entities[0].Mode != "pursuing-player" {
				t.Fatalf("enemy mode = %q, want pursuing-player", current.Entities[0].Mode)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("enemy did not move independently before timeout")
}

func TestStateIsReturnedAsSnapshot(t *testing.T) {
	manager := newTestManager(t)
	state := newTestGame(t, manager)
	state.Player.HP = 0

	current, err := manager.GetState(state.SessionID)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if current.Player.HP != 3 {
		t.Fatalf("mutating returned state changed stored HP to %d", current.Player.HP)
	}
}

func mustMove(t *testing.T, manager *GameManager, sessionID, direction string) *model.GameState {
	t.Helper()
	state, err := manager.Move(sessionID, direction)
	if err != nil {
		t.Fatalf("Move(%q) error = %v", direction, err)
	}
	return state
}

func groundTile(row, column int) model.Tile {
	return model.Tile{
		Row: row, Column: column, TypeID: "#TM000", TypeName: "ground",
		DisplayElement: "_", Walkable: true,
	}
}

func TestTerrainEffectsApplyToEveryEntityType(t *testing.T) {
	entityTypes := []string{"player", "bear", "gazelle"}
	effects := []struct {
		name  string
		setup func(*model.GameState)
		want  [2]int
		code  string
	}{
		{
			name: "ice",
			setup: func(state *model.GameState) {
				state.Grid[2][2] = effectTile(2, 2, "ice", "~")
			},
			want: [2]int{2, 3}, code: moveIce,
		},
		{
			name: "conveyor",
			setup: func(state *model.GameState) {
				state.Grid[2][2] = effectTile(2, 2, "conveyor", "»")
			},
			want: [2]int{2, 3}, code: moveConveyor,
		},
		{
			name: "portal",
			setup: func(state *model.GameState) {
				state.Grid[2][2] = effectTile(2, 2, "portal", "◈")
				state.Grid[2][2].PortalID = "pair-1"
				state.Grid[3][4] = effectTile(3, 4, "portal", "◇")
				state.Grid[3][4].PortalID = "pair-1"
			},
			want: [2]int{3, 4}, code: movePortal,
		},
	}

	for _, effect := range effects {
		for _, entityType := range entityTypes {
			t.Run(effect.name+"/"+entityType, func(t *testing.T) {
				state := openTestState(6, 7)
				effect.setup(state)
				entity := model.Entity{ID: entityType, Type: entityType, Row: 2, Column: 1, HP: 3, MaxHP: 3, Alive: true}
				result := moveEntity(state, &entity, 0, 1)
				if !result.Moved || result.Code != effect.code {
					t.Fatalf("moveEntity() = %+v, want moved with code %q", result, effect.code)
				}
				if entity.Row != effect.want[0] || entity.Column != effect.want[1] {
					t.Fatalf("entity position = (%d,%d), want (%d,%d)", entity.Row, entity.Column, effect.want[0], effect.want[1])
				}
			})
		}
	}
}

func TestSnowRequiresTwoAttemptsForEveryEntityType(t *testing.T) {
	for _, entityType := range []string{"player", "bear", "gazelle"} {
		t.Run(entityType, func(t *testing.T) {
			state := openTestState(5, 6)
			state.Grid[2][3] = effectTile(2, 3, "snow", "*")
			entity := model.Entity{ID: entityType, Type: entityType, Row: 2, Column: 2, HP: 3, Alive: true}

			first := moveEntity(state, &entity, 0, 1)
			if first.Moved || first.Code != moveSnowWait || entity.Column != 2 {
				t.Fatalf("first snow attempt = %+v at column %d", first, entity.Column)
			}
			second := moveEntity(state, &entity, 0, 1)
			if !second.Moved || second.Code != moveSnow || entity.Column != 3 {
				t.Fatalf("second snow attempt = %+v at column %d, want column 3", second, entity.Column)
			}
		})
	}
}

func TestHoleDamagesEveryEntityType(t *testing.T) {
	for _, entityType := range []string{"player", "bear", "gazelle"} {
		t.Run(entityType, func(t *testing.T) {
			state := openTestState(5, 6)
			state.Grid[2][2] = effectTile(2, 2, "hole", "○")
			state.Grid[2][2].HoleVisits = 1
			entity := model.Entity{ID: entityType, Type: entityType, Row: 2, Column: 1, HP: 2, MaxHP: 2, Alive: true}

			result := moveEntity(state, &entity, 0, 1)
			if result.Code != moveHoleDamage || entity.HP != 1 || state.Grid[2][2].Walkable {
				t.Fatalf("hole result/entity/tile = %+v/%+v/%+v", result, entity, state.Grid[2][2])
			}
		})
	}
}

func TestRampOrientationAppliesToEveryEntityType(t *testing.T) {
	for _, entityType := range []string{"player", "bear", "gazelle"} {
		t.Run(entityType, func(t *testing.T) {
			state := openTestState(6, 7)
			state.Grid[3][2] = model.Tile{
				Row: 3, Column: 2, TypeName: "ramp", DisplayElement: "→",
				Walkable: true, RampDirection: "east",
			}
			state.Grid[3][3] = model.Tile{
				Row: 3, Column: 3, TypeName: "high", DisplayElement: "▲",
				Walkable: true, Elevation: 1,
			}

			climber := model.Entity{ID: entityType, Type: entityType, Row: 3, Column: 1, HP: 3, Alive: true}
			if result := moveEntity(state, &climber, 0, 1); !result.Moved {
				t.Fatalf("entity could not enter ramp from the front: %+v", result)
			}
			if result := moveEntity(state, &climber, 0, 1); !result.Moved || climber.Elevation != 1 {
				t.Fatalf("entity could not reach high ground through ramp: %+v, elevation %d", result, climber.Elevation)
			}

			sideEntry := model.Entity{ID: entityType, Type: entityType, Row: 2, Column: 2, HP: 3, Alive: true}
			if result := moveEntity(state, &sideEntry, 1, 0); result.Moved || result.Code != moveRampSide {
				t.Fatalf("entity entered ramp from the side: %+v", result)
			}

			directClimb := model.Entity{ID: entityType, Type: entityType, Row: 2, Column: 3, HP: 3, Alive: true}
			if result := moveEntity(state, &directClimb, 1, 0); result.Moved || result.Code != moveHigh {
				t.Fatalf("entity climbed high ground without ramp: %+v", result)
			}

			descender := model.Entity{ID: entityType, Type: entityType, Row: 3, Column: 3, Elevation: 1, HP: 3, Alive: true}
			if result := moveEntity(state, &descender, 0, -1); !result.Moved {
				t.Fatalf("entity could not descend along ramp axis: %+v", result)
			}
		})
	}
}

func TestGazelleWanders(t *testing.T) {
	manager := newTestManager(t)
	state := openTestState(6, 7)
	state.Entities = []model.Entity{{
		ID: "gazelle-1", Type: "gazelle", Mode: "wandering",
		Row: 2, Column: 2, HP: 1, MaxHP: 1, Alive: true,
	}}

	manager.moveGazelles(state)
	gazelle := state.Entities[0]
	if gazelle.Row == 2 && gazelle.Column == 2 {
		t.Fatal("gazelle did not wander")
	}
	if gazelle.Mode != "wandering" {
		t.Fatalf("gazelle mode = %q, want wandering", gazelle.Mode)
	}
}

func TestBearCanChooseGazelleWithinThreeTiles(t *testing.T) {
	manager := newTestManager(t)
	state := openTestState(7, 8)
	state.EnemyAggression = 100
	state.Player = model.Entity{ID: "player", Type: "player", Row: 5, Column: 5, HP: 3, Alive: true}
	state.Entities = []model.Entity{
		{ID: "bear-1", Type: "bear", Hostile: true, Row: 2, Column: 2, HP: 5, Alive: true},
		{ID: "gazelle-1", Type: "gazelle", Row: 2, Column: 5, HP: 1, Alive: true},
	}

	manager.moveBears(state)
	if state.Entities[0].Mode != "hunting-gazelle" || state.Entities[0].Column != 3 {
		t.Fatalf("bear after decision = %+v, want gazelle hunt toward column 3", state.Entities[0])
	}
	manager.moveBears(state)
	manager.moveBears(state)
	resolvePredation(state)
	if state.Entities[1].Alive {
		t.Fatal("gazelle remained alive after bear reached it")
	}
}

func TestBearIgnoresGazelleOutsideDetectionRadius(t *testing.T) {
	manager := newTestManager(t)
	state := openTestState(7, 9)
	state.EnemyAggression = 100
	state.Player = model.Entity{ID: "player", Type: "player", Row: 2, Column: 1, HP: 3, Alive: true}
	state.Entities = []model.Entity{
		{ID: "bear-1", Type: "bear", Hostile: true, Row: 2, Column: 2, HP: 5, Alive: true},
		{ID: "gazelle-1", Type: "gazelle", Row: 2, Column: 6, HP: 1, Alive: true},
	}

	manager.moveBears(state)
	if state.Entities[0].Mode != "pursuing-player" {
		t.Fatalf("bear mode = %q, want pursuing-player", state.Entities[0].Mode)
	}
}

func TestBearCanDeclineNearbyGazelle(t *testing.T) {
	manager := newTestManager(t)
	state := openTestState(7, 8)
	state.EnemyAggression = 0
	state.Player = model.Entity{ID: "player", Type: "player", Row: 5, Column: 5, HP: 3, Alive: true}
	state.Entities = []model.Entity{
		{ID: "bear-1", Type: "bear", Hostile: true, Row: 2, Column: 2, HP: 5, Alive: true},
		{ID: "gazelle-1", Type: "gazelle", Row: 2, Column: 5, HP: 1, Alive: true},
	}

	manager.moveBears(state)
	if state.Entities[0].Mode != "patrolling" {
		t.Fatalf("bear mode = %q, want patrolling when hunt chance is 0", state.Entities[0].Mode)
	}
}

func openTestState(rows, columns int) *model.GameState {
	grid := make([][]model.Tile, rows)
	for row := range grid {
		grid[row] = make([]model.Tile, columns)
		for column := range grid[row] {
			grid[row][column] = groundTile(row, column)
		}
	}
	return &model.GameState{
		Rows: rows, Columns: columns, Grid: grid,
		Player: model.Entity{ID: "player", Type: "player", Row: 1, Column: 1, HP: 3, Alive: true},
		Status: "playing",
	}
}

func effectTile(row, column int, name, display string) model.Tile {
	return model.Tile{
		Row: row, Column: column, TypeName: name, DisplayElement: display, Walkable: true,
	}
}
