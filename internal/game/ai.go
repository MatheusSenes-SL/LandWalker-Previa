package game

import (
	"landwalker/internal/model"
	mathrand "math/rand"
)

const neutralHuntRadius = 3

func (manager *GameManager) moveGazelles(state *model.GameState) {
	for index := range state.Entities {
		gazelle := &state.Entities[index]
		if gazelle.Type != "gazelle" || !gazelle.Alive {
			continue
		}
		gazelle.Mode = "wandering"
		result := moveRandomly(state, gazelle, manager.rng)
		state.Revision++
		if !gazelle.Alive && result.Code == moveHoleDeath {
			state.Message = "🦌 The gazelle fell into a collapsed hole."
		}
	}
}

func (manager *GameManager) moveBears(state *model.GameState) {
	for index := range state.Entities {
		bear := &state.Entities[index]
		if bear.Type != "bear" || !bear.Alive {
			continue
		}

		var result movementResult
		prey := nearestNonHostile(state, *bear, neutralHuntRadius)
		if prey != nil {
			if manager.rng.Intn(100) < state.EnemyAggression {
				bear.Mode = "hunting-" + prey.Type
				result = moveTowards(state, bear, *prey)
			} else {
				bear.Mode = "patrolling"
				result = moveRandomly(state, bear, manager.rng)
			}
		} else if manager.rng.Intn(100) < state.EnemyAggression {
			bear.Mode = "pursuing-player"
			result = moveTowards(state, bear, state.Player)
		} else {
			bear.Mode = "patrolling"
			result = moveRandomly(state, bear, manager.rng)
		}

		state.Revision++
		if !bear.Alive && result.Code == moveHoleDeath {
			state.Message = "🐻 The bear fell into a collapsed hole."
		}
	}
}

func moveRandomly(state *model.GameState, entity *model.Entity, rng *mathrand.Rand) movementResult {
	directions := availableDirections(state, *entity)
	if len(directions) == 0 {
		return movementResult{Code: moveBlocked}
	}
	direction := directions[rng.Intn(len(directions))]
	return moveEntity(state, entity, direction[0], direction[1])
}

func moveTowards(state *model.GameState, entity *model.Entity, target model.Entity) movementResult {
	bestDirection := [2]int{}
	bestDistance := distance(entity.Row, entity.Column, target.Row, target.Column)
	for _, direction := range availableDirections(state, *entity) {
		row := entity.Row + direction[0]
		column := entity.Column + direction[1]
		candidateDistance := distance(row, column, target.Row, target.Column)
		if candidateDistance < bestDistance {
			bestDirection = direction
			bestDistance = candidateDistance
		}
	}
	if bestDirection == [2]int{} {
		return movementResult{Code: moveBlocked}
	}
	return moveEntity(state, entity, bestDirection[0], bestDirection[1])
}

func availableDirections(state *model.GameState, entity model.Entity) [][2]int {
	directions := neighboringDirections()
	available := make([][2]int, 0, len(directions))
	for _, direction := range directions {
		candidate := entity
		if moveOne(state, &candidate, direction[0], direction[1]).Moved {
			available = append(available, direction)
		}
	}
	return available
}

func nearestNonHostile(state *model.GameState, bear model.Entity, radius int) *model.Entity {
	var nearest *model.Entity
	nearestDistance := radius + 1
	for index := range state.Entities {
		candidate := &state.Entities[index]
		if candidate.ID == bear.ID || candidate.Hostile || !candidate.Alive {
			continue
		}
		candidateDistance := distance(bear.Row, bear.Column, candidate.Row, candidate.Column)
		if candidateDistance <= radius && candidateDistance < nearestDistance {
			nearest = candidate
			nearestDistance = candidateDistance
		}
	}
	return nearest
}

func resolvePredation(state *model.GameState) {
	for bearIndex := range state.Entities {
		bear := &state.Entities[bearIndex]
		if bear.Type != "bear" || !bear.Alive {
			continue
		}
		if samePosition(*bear, state.Player) {
			finishPlayerDeath(state, "🐻 The bear caught you! Game over.")
		}
		for preyIndex := range state.Entities {
			prey := &state.Entities[preyIndex]
			if prey.ID != bear.ID && !prey.Hostile && prey.Alive && samePosition(*bear, *prey) {
				prey.Alive = false
				prey.Mode = "hunted"
				state.Message = "🐻 The bear hunted the " + prey.Type + "."
				state.Revision++
			}
		}
	}
}

func neighboringDirections() [][2]int {
	return [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
}

func spawnBear(grid [][]model.Tile, playerRow, playerColumn int, door []int, rng *mathrand.Rand) (int, int) {
	return spawnBearAvoiding(grid, playerRow, playerColumn, door, nil, rng)
}

func spawnBearAvoiding(grid [][]model.Tile, playerRow, playerColumn int, door []int, occupied [][2]int, rng *mathrand.Rand) (int, int) {
	minimumDistance := (len(grid) + len(grid[0])) / 3
	if minimumDistance < 4 {
		minimumDistance = 4
	}
	blocked := append(append([][2]int(nil), occupied...), [2]int{playerRow, playerColumn})
	candidates, fallback := spawnCandidates(grid, blocked)
	preferred := make([][2]int, 0, len(candidates))
	for _, position := range candidates {
		if distance(position[0], position[1], playerRow, playerColumn) >= minimumDistance &&
			distance(position[0], position[1], door[0], door[1]) >= 2 {
			preferred = append(preferred, position)
		}
	}
	return choosePosition(preferred, fallback, rng)
}

func spawnGazelle(grid [][]model.Tile, playerRow, playerColumn, bearRow, bearColumn int, rng *mathrand.Rand) (int, int) {
	occupied := [][2]int{{playerRow, playerColumn}, {bearRow, bearColumn}}
	return spawnGazelleAvoiding(grid, playerRow, playerColumn, occupied, rng)
}

func spawnGazelleAvoiding(grid [][]model.Tile, playerRow, playerColumn int, occupied [][2]int, rng *mathrand.Rand) (int, int) {
	candidates, fallback := spawnCandidates(grid, occupied)
	preferred := make([][2]int, 0, len(candidates))
	for _, position := range candidates {
		farFromBears := true
		for _, blocked := range occupied {
			if blocked != [2]int{playerRow, playerColumn} && distance(position[0], position[1], blocked[0], blocked[1]) < 4 {
				farFromBears = false
				break
			}
		}
		if farFromBears && distance(position[0], position[1], playerRow, playerColumn) >= 2 {
			preferred = append(preferred, position)
		}
	}
	return choosePosition(preferred, fallback, rng)
}

func spawnCandidates(grid [][]model.Tile, occupied [][2]int) ([][2]int, [][2]int) {
	candidates := make([][2]int, 0)
	for row := 1; row < len(grid)-1; row++ {
		for column := 1; column < len(grid[row])-1; column++ {
			if grid[row][column].TypeName != "ground" || positionOccupied(row, column, occupied) {
				continue
			}
			candidates = append(candidates, [2]int{row, column})
		}
	}
	return candidates, candidates
}

func choosePosition(preferred, fallback [][2]int, rng *mathrand.Rand) (int, int) {
	positions := preferred
	if len(positions) == 0 {
		positions = fallback
	}
	position := positions[rng.Intn(len(positions))]
	return position[0], position[1]
}

func positionOccupied(row, column int, positions [][2]int) bool {
	for _, position := range positions {
		if position[0] == row && position[1] == column {
			return true
		}
	}
	return false
}

func distance(rowA, columnA, rowB, columnB int) int {
	return absolute(rowA-rowB) + absolute(columnA-columnB)
}

func absolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
