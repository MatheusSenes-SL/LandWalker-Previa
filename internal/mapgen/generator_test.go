package mapgen

import (
	"landwalker/internal/model"
	"reflect"
	"testing"
)

func TestGenerateIsDeterministicWithSeed(t *testing.T) {
	seed := int64(42)
	request := model.MapRequest{Rows: 10, Columns: 12, Seed: &seed, Difficulty: "hard"}

	first, err := Generate(request)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	second, err := Generate(request)
	if err != nil {
		t.Fatalf("Generate() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("Generate() returned different maps for the same seed")
	}
}

func TestGenerateCustomTerrainAndPortalPairs(t *testing.T) {
	seed := int64(7)
	generated, err := Generate(model.MapRequest{
		Rows: 10, Columns: 12, Seed: &seed,
		ElementCounts: map[string]int{"ice": 3, "portal": 2},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	counts := make(map[string]int)
	portalCounts := make(map[string]int)
	for _, row := range generated.Grid {
		for _, tile := range row {
			counts[tile.TypeName]++
			if tile.TypeName == "portal" {
				portalCounts[tile.PortalID]++
			}
		}
	}
	if counts["ice"] != 3 || counts["portal"] != 4 {
		t.Fatalf("terrain counts ice/portal = %d/%d, want 3/4", counts["ice"], counts["portal"])
	}
	for id, count := range portalCounts {
		if count != 2 {
			t.Fatalf("portal pair %q has %d tiles, want 2", id, count)
		}
	}
}

func TestGenerateRejectsInvalidInput(t *testing.T) {
	tests := []model.MapRequest{
		{Rows: 4, Columns: 10},
		{Rows: 10, Columns: 31},
		{Rows: 10, Columns: 21},
		{Rows: 10, Columns: 10, Difficulty: "impossible"},
		{Rows: 5, Columns: 5, ElementCounts: map[string]int{"wall": 100}},
		{Rows: 10, Columns: 10, ElementCounts: map[string]int{"unknown": 1}},
	}
	for _, request := range tests {
		if _, err := Generate(request); err == nil {
			t.Fatalf("Generate(%+v) unexpectedly succeeded", request)
		}
	}
}

func TestGenerateAllowsExactlyTwoHundredTiles(t *testing.T) {
	generated, err := Generate(model.MapRequest{Rows: 10, Columns: 20})
	if err != nil {
		t.Fatalf("Generate(10x20) error = %v", err)
	}
	if generated.Rows*generated.Columns != MaxTiles {
		t.Fatalf("generated area = %d, want %d", generated.Rows*generated.Columns, MaxTiles)
	}
}

func TestGeneratedRampFacesHighGroundAndHasClearFront(t *testing.T) {
	seed := int64(99)
	generated, err := Generate(model.MapRequest{Rows: 12, Columns: 16, Seed: &seed})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	for row := range generated.Grid {
		for column, tile := range generated.Grid[row] {
			if tile.TypeName != "ramp" {
				continue
			}
			delta := map[string][2]int{
				"north": {-1, 0}, "east": {0, 1}, "south": {1, 0}, "west": {0, -1},
			}[tile.RampDirection]
			high := generated.Grid[row+delta[0]][column+delta[1]]
			front := generated.Grid[row-delta[0]][column-delta[1]]
			if high.TypeName != "high" {
				t.Fatalf("ramp at (%d,%d) faces %q instead of high ground", row, column, high.TypeName)
			}
			if front.TypeName != "ground" || !front.Walkable {
				t.Fatalf("ramp front at (%d,%d) is not clear ground: %+v", row-delta[0], column-delta[1], front)
			}
			return
		}
	}
	t.Fatal("Generate() did not create a ramp")
}
