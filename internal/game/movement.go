package game

import (
	"fmt"
	"landwalker/internal/model"
)

const (
	moveOK         = "moved"
	moveBoundary   = "boundary"
	moveBlocked    = "blocked"
	moveHigh       = "high"
	moveRampSide   = "ramp-side"
	moveSnowWait   = "snow-wait"
	moveSnow       = "snow"
	moveHoleCrack  = "hole-crack"
	moveHoleDamage = "hole-damage"
	moveHoleDeath  = "hole-death"
	moveIce        = "ice"
	moveConveyor   = "conveyor"
	movePortal     = "portal"
	maxForcedMoves = 64
)

type movementResult struct {
	Moved       bool
	Code        string
	BlockedTile string
}

// moveEntity is the single movement pipeline for players and NPCs.
func moveEntity(state *model.GameState, entity *model.Entity, deltaRow, deltaColumn int) movementResult {
	if !entity.Alive {
		return movementResult{Code: moveBlocked}
	}

	targetRow := entity.Row + deltaRow
	targetColumn := entity.Column + deltaColumn
	direction := directionName(deltaRow, deltaColumn)
	if inside(state, targetRow, targetColumn) {
		target := state.Grid[targetRow][targetColumn]
		if target.TypeName == "snow" && target.Walkable {
			if entity.SnowSteps == 0 || entity.SnowDirection != direction {
				entity.SnowSteps = 1
				entity.SnowDirection = direction
				return movementResult{Code: moveSnowWait}
			}
		}
	}
	entity.SnowSteps = 0
	entity.SnowDirection = ""

	result := moveOne(state, entity, deltaRow, deltaColumn)
	if !result.Moved {
		return result
	}

	for forcedMoves := 0; forcedMoves < maxForcedMoves && entity.Alive; forcedMoves++ {
		tile := &state.Grid[entity.Row][entity.Column]
		switch tile.TypeName {
		case "hole":
			tile.HoleVisits++
			if tile.HoleVisits == 1 {
				result.Code = moveHoleCrack
				return result
			}
			tile.Walkable = false
			tile.DisplayElement = "☠"
			entity.HP--
			if entity.HP <= 0 {
				entity.Alive = false
				result.Code = moveHoleDeath
			} else {
				result.Code = moveHoleDamage
			}
			return result
		case "ice":
			next := moveOne(state, entity, deltaRow, deltaColumn)
			result.Code = moveIce
			if !next.Moved {
				return result
			}
		case "conveyor":
			next := moveOne(state, entity, 0, 1)
			result.Code = moveConveyor
			if !next.Moved {
				return result
			}
			deltaRow, deltaColumn = 0, 1
		case "portal":
			row, column := findPairPortal(state.Grid, entity.Row, entity.Column, tile.PortalID)
			if row >= 0 {
				entity.Row, entity.Column = row, column
				entity.Elevation = state.Grid[row][column].Elevation
				result.Code = movePortal
			}
			return result
		case "snow":
			result.Code = moveSnow
			return result
		default:
			return result
		}
	}
	return result
}

func moveOne(state *model.GameState, entity *model.Entity, deltaRow, deltaColumn int) movementResult {
	targetRow := entity.Row + deltaRow
	targetColumn := entity.Column + deltaColumn
	if !inside(state, targetRow, targetColumn) {
		return movementResult{Code: moveBoundary}
	}

	current := state.Grid[entity.Row][entity.Column]
	target := state.Grid[targetRow][targetColumn]
	if !target.Walkable {
		return movementResult{Code: moveBlocked, BlockedTile: target.TypeName}
	}
	if rampBlocked(current, target, entity.Elevation, deltaRow, deltaColumn) {
		return movementResult{Code: moveRampSide}
	}
	if target.Elevation > entity.Elevation {
		if current.TypeName != "ramp" || current.RampDirection != directionName(deltaRow, deltaColumn) {
			return movementResult{Code: moveHigh}
		}
	}

	entity.Row, entity.Column = targetRow, targetColumn
	entity.Elevation = target.Elevation
	return movementResult{Moved: true, Code: moveOK}
}

func rampBlocked(current, target model.Tile, elevation, deltaRow, deltaColumn int) bool {
	direction := directionName(deltaRow, deltaColumn)
	if current.TypeName == "ramp" &&
		direction != current.RampDirection &&
		direction != oppositeDirection(current.RampDirection) {
		return true
	}
	if target.TypeName != "ramp" {
		return false
	}
	if target.RampDirection == "" {
		return true
	}
	if elevation > target.Elevation {
		return direction != oppositeDirection(target.RampDirection)
	}
	return direction != target.RampDirection
}

func directionName(deltaRow, deltaColumn int) string {
	switch {
	case deltaRow < 0:
		return "north"
	case deltaRow > 0:
		return "south"
	case deltaColumn < 0:
		return "west"
	case deltaColumn > 0:
		return "east"
	default:
		return ""
	}
}

func oppositeDirection(direction string) string {
	switch direction {
	case "north":
		return "south"
	case "south":
		return "north"
	case "east":
		return "west"
	case "west":
		return "east"
	default:
		return ""
	}
}

func inside(state *model.GameState, row, column int) bool {
	return row >= 0 && row < state.Rows && column >= 0 && column < state.Columns
}

func findPairPortal(grid [][]model.Tile, currentRow, currentColumn int, portalID string) (int, int) {
	for row := range grid {
		for column := range grid[row] {
			tile := grid[row][column]
			if (row != currentRow || column != currentColumn) && tile.TypeName == "portal" && tile.PortalID == portalID {
				return row, column
			}
		}
	}
	return -1, -1
}

func playerMovementMessage(result movementResult) string {
	switch result.Code {
	case moveBoundary:
		return "You hit the edge of the world!"
	case moveBlocked:
		return fmt.Sprintf("Blocked by %s!", result.BlockedTile)
	case moveHigh:
		return "You can only reach high ground through a ramp."
	case moveRampSide:
		return "The ramp only works from the front. Follow its arrow."
	case moveSnowWait:
		return "The deep snow slows your entry. Repeat the move to push through."
	case moveSnow:
		return "You pushed through the deep snow."
	case moveHoleCrack:
		return "The hole is cracking. Do not step here again!"
	case moveHoleDamage:
		return "The hole collapsed! You took 1 damage."
	case moveHoleDeath:
		return "The hole collapsed completely. Game over."
	case moveIce:
		return "You slid across the ice!"
	case moveConveyor:
		return "The conveyor carried you east!"
	case movePortal:
		return "✨ An ancient portal teleported you!"
	default:
		return "Moved."
	}
}
