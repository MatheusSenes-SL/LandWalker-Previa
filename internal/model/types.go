package model

type Tile struct {
	Position       int    `json:"position"`
	Row            int    `json:"row"`
	Column         int    `json:"column"`
	TypeID         string `json:"typeID"`
	TypeName       string `json:"typeName"`
	DisplayElement string `json:"displayElement"`
	Walkable       bool   `json:"walkable"`
	Elevation      int    `json:"elevation"`
	HasNorth       bool   `json:"hasNorth"`
	HasEast        bool   `json:"hasEast"`
	HasSouth       bool   `json:"hasSouth"`
	HasWest        bool   `json:"hasWest"`
	HoleVisits     int    `json:"holeVisits,omitempty"`
	PortalID       string `json:"portalId,omitempty"`
	RampDirection  string `json:"rampDirection,omitempty"`
}

type MapResponse struct {
	Rows          int      `json:"rows"`
	Columns       int      `json:"columns"`
	Seed          int64    `json:"seed"`
	Grid          [][]Tile `json:"grid"`
	StartPosition []int    `json:"startPosition"`
	DoorPosition  []int    `json:"doorPosition"`
}

type MapRequest struct {
	Rows          int            `json:"rows"`
	Columns       int            `json:"columns"`
	Seed          *int64         `json:"seed,omitempty"`
	Difficulty    string         `json:"difficulty"`
	ElementCounts map[string]int `json:"elementCounts,omitempty"`
}

type Entity struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Mode          string `json:"mode,omitempty"`
	Hostile       bool   `json:"hostile"`
	Row           int    `json:"row"`
	Column        int    `json:"column"`
	HP            int    `json:"hp"`
	MaxHP         int    `json:"maxHp"`
	Alive         bool   `json:"alive"`
	Elevation     int    `json:"elevation"`
	SnowSteps     int    `json:"snowSteps,omitempty"`
	SnowDirection string `json:"snowDirection,omitempty"`
}

type GameState struct {
	SessionID       string         `json:"sessionId"`
	Revision        uint64         `json:"revision"`
	Level           int            `json:"level"`
	TotalLevels     int            `json:"totalLevels"`
	Rows            int            `json:"rows"`
	Columns         int            `json:"columns"`
	Seed            int64          `json:"seed"`
	Difficulty      string         `json:"difficulty"`
	EnemyAggression int            `json:"enemyAggression"`
	ElementCounts   map[string]int `json:"elementCounts,omitempty"`
	Grid            [][]Tile       `json:"grid"`
	Player          Entity         `json:"player"`
	Entities        []Entity       `json:"entities"`
	MovesCount      int            `json:"movesCount"`
	GameOver        bool           `json:"gameOver"`
	GameWon         bool           `json:"gameWon"`
	Message         string         `json:"message"`
	Door            []int          `json:"door"`
	Status          string         `json:"status"`
	EngineType      string         `json:"engineType"`
}

type MoveRequest struct {
	Direction string `json:"direction"`
}

type NewGameRequest struct {
	Rows            int            `json:"rows"`
	Columns         int            `json:"columns"`
	Difficulty      string         `json:"difficulty"`
	TotalLevels     int            `json:"totalLevels,omitempty"`
	EnemyAggression *int           `json:"enemyAggression,omitempty"`
	Seed            *int64         `json:"seed,omitempty"`
	ElementCounts   map[string]int `json:"elementCounts,omitempty"`
}
