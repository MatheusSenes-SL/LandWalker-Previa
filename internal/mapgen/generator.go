package mapgen

import (
	"fmt"
	"landwalker/internal/model"
	"math/rand"
	"strings"
	"time"
)

const (
	DefaultRows       = 12
	DefaultColumns    = 16
	DefaultDifficulty = "normal"
	MinSize           = 5
	MaxSize           = 30
	MaxTiles          = 200
)

type tileKind struct {
	display   string
	id        string
	name      string
	walkable  bool
	elevation int
}

var kinds = map[string]tileKind{
	"ground":   {"_", "#TM000", "ground", true, 0},
	"door":     {"⊕", "#TM001", "door", true, 0},
	"high":     {"▲", "#TM002", "high", true, 1},
	"ramp":     {"↑", "#TM003", "ramp", true, 0},
	"ice":      {"~", "#TM004", "ice", true, 0},
	"water":    {"≈", "#TM005", "water", false, 0},
	"hole":     {"○", "#TM901", "hole", true, 0},
	"conveyor": {"»", "#SP300", "conveyor", true, 0},
	"snow":     {"*", "#SP801", "snow", true, 0},
	"wall":     {"#", "#TM900", "wall", false, 0},
}

var customKinds = []string{"ice", "water", "hole", "conveyor", "snow", "wall"}

// Generate creates a complete map using only Go's standard library.
func Generate(req model.MapRequest) (*model.MapResponse, error) {
	rows, columns, difficulty, err := normalize(req.Rows, req.Columns, req.Difficulty)
	if err != nil {
		return nil, err
	}

	seed := time.Now().UnixNano()
	if req.Seed != nil {
		seed = *req.Seed
	}
	rng := rand.New(rand.NewSource(seed))
	grid := baseGrid(rows, columns)
	start := []int{rows / 2, columns / 2}
	door := placeDoor(grid, rng)
	reserved := reservedCells(start, door, rows, columns)

	if len(req.ElementCounts) == 0 {
		placeDefaultTerrain(grid, difficulty, reserved, rng)
	} else if err := placeCustomTerrain(grid, req.ElementCounts, reserved, rng); err != nil {
		return nil, err
	}

	linkDirections(grid)
	return &model.MapResponse{
		Rows:          rows,
		Columns:       columns,
		Seed:          seed,
		Grid:          grid,
		StartPosition: start,
		DoorPosition:  door,
	}, nil
}

func normalize(rows, columns int, difficulty string) (int, int, string, error) {
	if rows == 0 {
		rows = DefaultRows
	}
	if columns == 0 {
		columns = DefaultColumns
	}
	if rows < MinSize || rows > MaxSize || columns < MinSize || columns > MaxSize {
		return 0, 0, "", fmt.Errorf("rows and columns must be between %d and %d", MinSize, MaxSize)
	}
	if rows*columns > MaxTiles {
		return 0, 0, "", fmt.Errorf("map area cannot exceed %d tiles", MaxTiles)
	}

	difficulty = strings.ToLower(strings.TrimSpace(difficulty))
	if difficulty == "" {
		difficulty = DefaultDifficulty
	}
	if difficulty != "easy" && difficulty != "normal" && difficulty != "hard" {
		return 0, 0, "", fmt.Errorf("difficulty must be easy, normal, or hard")
	}
	return rows, columns, difficulty, nil
}

func baseGrid(rows, columns int) [][]model.Tile {
	grid := make([][]model.Tile, rows)
	for row := range grid {
		grid[row] = make([]model.Tile, columns)
		for column := range grid[row] {
			kind := kinds["ground"]
			if row == 0 || row == rows-1 || column == 0 || column == columns-1 {
				kind = kinds["wall"]
			}
			grid[row][column] = newTile(row, column, columns, kind)
		}
	}
	return grid
}

func newTile(row, column, columns int, kind tileKind) model.Tile {
	return model.Tile{
		Position:       row*columns + column,
		Row:            row,
		Column:         column,
		TypeID:         kind.id,
		TypeName:       kind.name,
		DisplayElement: kind.display,
		Walkable:       kind.walkable,
		Elevation:      kind.elevation,
	}
}

func setKind(tile *model.Tile, kind tileKind) {
	tile.TypeID = kind.id
	tile.TypeName = kind.name
	tile.DisplayElement = kind.display
	tile.Walkable = kind.walkable
	tile.Elevation = kind.elevation
	tile.HoleVisits = 0
	tile.PortalID = ""
	tile.RampDirection = ""
}

func placeDoor(grid [][]model.Tile, rng *rand.Rand) []int {
	rows, columns := len(grid), len(grid[0])
	positions := make([][2]int, 0, 2*(rows+columns-4))
	for row := 1; row < rows-1; row++ {
		positions = append(positions, [2]int{row, 0}, [2]int{row, columns - 1})
	}
	for column := 1; column < columns-1; column++ {
		positions = append(positions, [2]int{0, column}, [2]int{rows - 1, column})
	}
	position := positions[rng.Intn(len(positions))]
	setKind(&grid[position[0]][position[1]], kinds["door"])
	return []int{position[0], position[1]}
}

func reservedCells(start, door []int, rows, columns int) map[[2]int]bool {
	reserved := make(map[[2]int]bool)
	for row := start[0] - 1; row <= start[0]+1; row++ {
		for column := start[1] - 1; column <= start[1]+1; column++ {
			reserved[[2]int{row, column}] = true
		}
	}
	reserved[[2]int{door[0], door[1]}] = true
	if door[0] == 0 {
		reserved[[2]int{1, door[1]}] = true
	} else if door[0] == rows-1 {
		reserved[[2]int{rows - 2, door[1]}] = true
	} else if door[1] == 0 {
		reserved[[2]int{door[0], 1}] = true
	} else if door[1] == columns-1 {
		reserved[[2]int{door[0], columns - 2}] = true
	}
	return reserved
}

func availableCells(grid [][]model.Tile, reserved map[[2]int]bool, rng *rand.Rand) [][2]int {
	positions := make([][2]int, 0)
	for row := 1; row < len(grid)-1; row++ {
		for column := 1; column < len(grid[row])-1; column++ {
			position := [2]int{row, column}
			if !reserved[position] && grid[row][column].TypeName == "ground" {
				positions = append(positions, position)
			}
		}
	}
	rng.Shuffle(len(positions), func(i, j int) { positions[i], positions[j] = positions[j], positions[i] })
	return positions
}

func placeDefaultTerrain(grid [][]model.Tile, difficulty string, reserved map[[2]int]bool, rng *rand.Rand) {
	positions := availableCells(grid, reserved, rng)
	if len(positions) >= 2 {
		high, ramp, front, found := rampPair(positions, grid)
		if found {
			setKind(&grid[high[0]][high[1]], kinds["high"])
			setKind(&grid[ramp[0]][ramp[1]], kinds["ramp"])
			orientRamp(&grid[ramp[0]][ramp[1]], ramp, high)
			positions = removeCoordinates(positions, high, ramp, front)
		}
	}

	density := map[string]int{"easy": 2, "normal": 3, "hard": 5}[difficulty]
	terrain := []string{"ice", "water", "hole", "conveyor", "snow", "wall"}
	for index := 0; index < density && len(positions) > 0; index++ {
		position := positions[0]
		positions = positions[1:]
		setKind(&grid[position[0]][position[1]], kinds[terrain[rng.Intn(len(terrain))]])
	}
	placePortalPair(grid, positions, "portal-1")
}

func placeCustomTerrain(grid [][]model.Tile, counts map[string]int, reserved map[[2]int]bool, rng *rand.Rand) error {
	required := 0
	for name, count := range counts {
		if count < 0 {
			return fmt.Errorf("element count for %q cannot be negative", name)
		}
		if name == "portal" {
			required += count * 2
			continue
		}
		if _, ok := kinds[name]; !ok || name == "ground" || name == "door" {
			return fmt.Errorf("unknown map element %q", name)
		}
		required += count
	}

	positions := availableCells(grid, reserved, rng)
	if required > len(positions) {
		return fmt.Errorf("requested %d terrain cells, but this map has room for %d", required, len(positions))
	}

	highRemaining := counts["high"]
	rampRemaining := counts["ramp"]
	for highRemaining > 0 && rampRemaining > 0 {
		high, ramp, front, found := rampPair(positions, grid)
		if !found {
			break
		}
		setKind(&grid[high[0]][high[1]], kinds["high"])
		setKind(&grid[ramp[0]][ramp[1]], kinds["ramp"])
		orientRamp(&grid[ramp[0]][ramp[1]], ramp, high)
		positions = removeCoordinates(positions, high, ramp, front)
		highRemaining--
		rampRemaining--
	}
	remainingRequired := highRemaining + rampRemaining + counts["portal"]*2
	for _, name := range customKinds {
		remainingRequired += counts[name]
	}
	if remainingRequired > len(positions) {
		return fmt.Errorf("the requested ramps need accessible front tiles; reduce the terrain counts")
	}
	for count := 0; count < highRemaining; count++ {
		position := positions[0]
		positions = positions[1:]
		setKind(&grid[position[0]][position[1]], kinds["high"])
	}
	directions := []string{"north", "east", "south", "west"}
	for count := 0; count < rampRemaining; count++ {
		position := positions[0]
		positions = positions[1:]
		setKind(&grid[position[0]][position[1]], kinds["ramp"])
		setRampDirection(&grid[position[0]][position[1]], directions[rng.Intn(len(directions))])
	}

	for _, name := range customKinds {
		for count := 0; count < counts[name]; count++ {
			position := positions[0]
			positions = positions[1:]
			setKind(&grid[position[0]][position[1]], kinds[name])
		}
	}
	for count := 0; count < counts["portal"]; count++ {
		placePortalPair(grid, positions[:2], fmt.Sprintf("portal-%d", count+1))
		positions = positions[2:]
	}
	return nil
}

func placePortalPair(grid [][]model.Tile, positions [][2]int, portalID string) {
	if len(positions) < 2 {
		return
	}
	first, second := positions[0], positions[1]
	portalOne := tileKind{"◈", "#SP301", "portal", true, 0}
	portalTwo := tileKind{"◇", "#SP302", "portal", true, 0}
	setKind(&grid[first[0]][first[1]], portalOne)
	setKind(&grid[second[0]][second[1]], portalTwo)
	grid[first[0]][first[1]].PortalID = portalID
	grid[second[0]][second[1]].PortalID = portalID
}

func rampPair(positions [][2]int, grid [][]model.Tile) (high, ramp, front [2]int, found bool) {
	for first := range positions {
		for second := first + 1; second < len(positions); second++ {
			if absolute(positions[first][0]-positions[second][0])+absolute(positions[first][1]-positions[second][1]) != 1 {
				continue
			}
			for _, pair := range [][2][2]int{{positions[first], positions[second]}, {positions[second], positions[first]}} {
				high, ramp = pair[0], pair[1]
				front = [2]int{2*ramp[0] - high[0], 2*ramp[1] - high[1]}
				if front[0] > 0 && front[0] < len(grid)-1 &&
					front[1] > 0 && front[1] < len(grid[front[0]])-1 &&
					grid[front[0]][front[1]].TypeName == "ground" {
					return high, ramp, front, true
				}
			}
		}
	}
	return high, ramp, front, false
}

func removeCoordinates(positions [][2]int, removed ...[2]int) [][2]int {
	filtered := make([][2]int, 0, len(positions)-len(removed))
	for _, position := range positions {
		remove := false
		for _, candidate := range removed {
			if position == candidate {
				remove = true
				break
			}
		}
		if !remove {
			filtered = append(filtered, position)
		}
	}
	return filtered
}

func orientRamp(tile *model.Tile, ramp, high [2]int) {
	switch {
	case high[0] < ramp[0]:
		setRampDirection(tile, "north")
	case high[0] > ramp[0]:
		setRampDirection(tile, "south")
	case high[1] < ramp[1]:
		setRampDirection(tile, "west")
	default:
		setRampDirection(tile, "east")
	}
}

func setRampDirection(tile *model.Tile, direction string) {
	tile.RampDirection = direction
	tile.DisplayElement = map[string]string{
		"north": "↑",
		"east":  "→",
		"south": "↓",
		"west":  "←",
	}[direction]
}

func absolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func linkDirections(grid [][]model.Tile) {
	for row := range grid {
		for column := range grid[row] {
			grid[row][column].HasNorth = row > 0
			grid[row][column].HasEast = column < len(grid[row])-1
			grid[row][column].HasSouth = row < len(grid)-1
			grid[row][column].HasWest = column > 0
		}
	}
}
