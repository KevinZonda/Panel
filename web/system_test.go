package web

import "testing"

func TestChunkCoresColumnTiers(t *testing.T) {
	cases := []struct {
		cores    int
		wantCols int
	}{
		{1, 4},
		{4, 4},
		{8, 4},
		{32, 4},
		{33, 8},
		{64, 8},
		{65, 16},
		{128, 16},
	}
	for _, tc := range cases {
		perCore := make([]float64, tc.cores)
		rows, cols := chunkCores(perCore)
		if len(rows) == 0 {
			t.Fatalf("cores %d: no rows", tc.cores)
		}
		if cols != tc.wantCols {
			t.Fatalf("cores %d: expected %d columns, got %d", tc.cores, tc.wantCols, cols)
		}
		maxRowWidth := 0
		total := 0
		for _, row := range rows {
			if len(row) > maxRowWidth {
				maxRowWidth = len(row)
			}
			total += len(row)
		}
		wantWidth := tc.wantCols
		if tc.cores < tc.wantCols {
			wantWidth = tc.cores
		}
		if maxRowWidth != wantWidth {
			t.Fatalf("cores %d: expected row width %d, got %d", tc.cores, wantWidth, maxRowWidth)
		}
		maxRows := (tc.cores + tc.wantCols - 1) / tc.wantCols
		if len(rows) != maxRows {
			t.Fatalf("cores %d: expected %d rows, got %d", tc.cores, maxRows, len(rows))
		}
		if total != tc.cores {
			t.Fatalf("cores %d: expected %d cells total, got %d", tc.cores, tc.cores, total)
		}
	}
}

func TestChunkCoresIndexesAreSequential(t *testing.T) {
	perCore := make([]float64, 10)
	rows, _ := chunkCores(perCore)
	want := 0
	for _, row := range rows {
		for _, cell := range row {
			if cell.Index != want {
				t.Fatalf("expected index %d, got %d", want, cell.Index)
			}
			want++
		}
	}
	if want != 10 {
		t.Fatalf("expected 10 cells, got %d", want)
	}
}

func TestChunkCoresKeepsTablesShort(t *testing.T) {
	// Design goal: even a 128-core machine fits in 8 rows.
	for _, cores := range []int{32, 64, 128} {
		rows, _ := chunkCores(make([]float64, cores))
		if len(rows) > 8 {
			t.Fatalf("cores %d: table has %d rows, want at most 8", cores, len(rows))
		}
	}
}
