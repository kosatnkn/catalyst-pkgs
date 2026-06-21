package postgres

import (
	"reflect"
	"regexp"
	"testing"
)

// newTestAdapter returns a bare adapter instance suitable for testing
// the pure helper methods (no DB connection required).
func newTestAdapter() *DatabaseAdapterPostgres {
	return &DatabaseAdapterPostgres{
		paramPrefix:  namedParamPrefix,
		paramDivider: namedParamDivider,
		paramExp:     regexp.MustCompile(namedParamRegex),
	}
}

// ─── isSelect ────────────────────────────────────────────────────────────────

func TestIsSelect(t *testing.T) {
	a := newTestAdapter()

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		// positive cases
		{"lowercase select", "select * from tbl", true},
		{"uppercase SELECT", "SELECT * FROM tbl", true},
		{"mixed case Select", "Select id FROM tbl", true},
		{"select with leading spaces trimmed by caller", "select col FROM tbl WHERE id = $1", true},

		// negative cases
		{"insert", "insert into tbl(col) values ($1)", false},
		{"update", "update tbl set col = $1", false},
		{"delete", "delete from tbl where id = $1", false},
		{"with CTE", "with cte as (select 1) select * from cte", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := a.isSelect(tc.query)
			if got != tc.want {
				t.Errorf("isSelect(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

// ─── isInsert ────────────────────────────────────────────────────────────────

func TestIsInsert(t *testing.T) {
	a := newTestAdapter()

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		// positive cases
		{"lowercase insert", "insert into tbl(col) values ($1)", true},
		{"uppercase INSERT", "INSERT INTO tbl(col) VALUES ($1)", true},
		{"mixed case Insert", "Insert INTO tbl(col) VALUES ($1)", true},

		// negative cases
		{"select", "select * from tbl", false},
		{"update", "update tbl set col = $1", false},
		{"delete", "delete from tbl where id = $1", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := a.isInsert(tc.query)
			if got != tc.want {
				t.Errorf("isInsert(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

// ─── convertQuery ─────────────────────────────────────────────────────────────

func TestConvertQuery(t *testing.T) {
	a := newTestAdapter()

	tests := []struct {
		name            string
		input           string
		wantQuery       string
		wantNamedParams []string
	}{
		// ── no params ────────────────────────────────────────────────────────
		{
			name:            "no params — query unchanged",
			input:           "SELECT * FROM tbl",
			wantQuery:       "SELECT * FROM tbl",
			wantNamedParams: nil,
		},

		// ── whitespace handling ───────────────────────────────────────────────
		{
			name:            "leading and trailing whitespace trimmed",
			input:           "  SELECT * FROM tbl WHERE id = ?id  ",
			wantQuery:       "SELECT * FROM tbl WHERE id = $1",
			wantNamedParams: []string{"id"},
		},
		{
			name:            "inner whitespace preserved",
			input:           "SELECT  *  FROM  tbl  WHERE  id = ?id",
			wantQuery:       "SELECT  *  FROM  tbl  WHERE  id = $1",
			wantNamedParams: []string{"id"},
		},

		// ── single param ─────────────────────────────────────────────────────
		{
			name:            "SELECT single param",
			input:           "SELECT * FROM tbl WHERE id = ?id",
			wantQuery:       "SELECT * FROM tbl WHERE id = $1",
			wantNamedParams: []string{"id"},
		},
		{
			name:            "DELETE single param",
			input:           "DELETE FROM tbl WHERE id = ?id",
			wantQuery:       "DELETE FROM tbl WHERE id = $1",
			wantNamedParams: []string{"id"},
		},

		// ── multiple params ───────────────────────────────────────────────────
		{
			name:            "INSERT multiple params",
			input:           "INSERT INTO tbl(a, b, c) VALUES (?a, ?b, ?c)",
			wantQuery:       "INSERT INTO tbl(a, b, c) VALUES ($1, $2, $3)",
			wantNamedParams: []string{"a", "b", "c"},
		},
		{
			name:            "UPDATE multiple params",
			input:           "UPDATE tbl SET col1 = ?col1, col2 = ?col2 WHERE id = ?id",
			wantQuery:       "UPDATE tbl SET col1 = $1, col2 = $2 WHERE id = $3",
			wantNamedParams: []string{"col1", "col2", "id"},
		},
		{
			name:            "SELECT multiple params in WHERE",
			input:           "SELECT * FROM tbl WHERE a = ?a AND b = ?b AND c = ?c",
			wantQuery:       "SELECT * FROM tbl WHERE a = $1 AND b = $2 AND c = $3",
			wantNamedParams: []string{"a", "b", "c"},
		},

		// ── param name formats ───────────────────────────────────────────────
		{
			name:            "param with underscore",
			input:           "SELECT * FROM tbl WHERE user_id = ?user_id",
			wantQuery:       "SELECT * FROM tbl WHERE user_id = $1",
			wantNamedParams: []string{"user_id"},
		},
		{
			name:            "param with leading underscore",
			input:           "SELECT * FROM tbl WHERE id = ?_id",
			wantQuery:       "SELECT * FROM tbl WHERE id = $1",
			wantNamedParams: []string{"_id"},
		},
		{
			name:            "param with digits",
			input:           "SELECT * FROM tbl WHERE col = ?col1",
			wantQuery:       "SELECT * FROM tbl WHERE col = $1",
			wantNamedParams: []string{"col1"},
		},
		{
			name:            "param that is all digits",
			input:           "SELECT * FROM tbl WHERE col = ?123",
			wantQuery:       "SELECT * FROM tbl WHERE col = $1",
			wantNamedParams: []string{"123"},
		},

		// ── repeated param ────────────────────────────────────────────────────
		{
			name:            "repeated param gets separate positional placeholders",
			input:           "SELECT * FROM tbl WHERE a = ?val OR b = ?val",
			wantQuery:       "SELECT * FROM tbl WHERE a = $1 OR b = $2",
			wantNamedParams: []string{"val", "val"},
		},

		// ── type suffix (#xxx) ────────────────────────────────────────────────
		{
			name:            "arr suffix — ANY clause",
			input:           "SELECT * FROM tbl WHERE id = ANY(?ids#arr)",
			wantQuery:       "SELECT * FROM tbl WHERE id = ANY($1)",
			wantNamedParams: []string{"ids#arr"},
		},
		{
			name:            "mixed — some params with suffix some without",
			input:           "SELECT * FROM tbl WHERE id = ANY(?ids#arr) AND name = ?name",
			wantQuery:       "SELECT * FROM tbl WHERE id = ANY($1) AND name = $2",
			wantNamedParams: []string{"ids#arr", "name"},
		},
		{
			name:            "multiple suffixed params",
			input:           "SELECT * FROM tbl WHERE id = ANY(?ids#arr) AND score > ?score#arr",
			wantQuery:       "SELECT * FROM tbl WHERE id = ANY($1) AND score > $2",
			wantNamedParams: []string{"ids#arr", "score#arr"},
		},

		// ── positional ordering ───────────────────────────────────────────────
		{
			name:            "positional order follows appearance in query",
			input:           "INSERT INTO tbl(z, a, m) VALUES (?z, ?a, ?m)",
			wantQuery:       "INSERT INTO tbl(z, a, m) VALUES ($1, $2, $3)",
			wantNamedParams: []string{"z", "a", "m"},
		},

		// ── postgres-specific syntax ──────────────────────────────────────────
		{
			name:            "param in ANY() clause",
			input:           "SELECT * FROM tbl WHERE id = ANY(?ids#arr)",
			wantQuery:       "SELECT * FROM tbl WHERE id = ANY($1)",
			wantNamedParams: []string{"ids#arr"},
		},
		{
			name:            "param alongside RETURNING clause",
			input:           "INSERT INTO tbl(col) VALUES (?val) RETURNING id",
			wantQuery:       "INSERT INTO tbl(col) VALUES ($1) RETURNING id",
			wantNamedParams: []string{"val"},
		},
		{
			name:            "param at end of string (no terminator)",
			input:           "SELECT * FROM tbl WHERE id = ?id",
			wantQuery:       "SELECT * FROM tbl WHERE id = $1",
			wantNamedParams: []string{"id"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotQuery, gotNamedParams := a.convertQuery(tc.input)

			if gotQuery != tc.wantQuery {
				t.Errorf("query\n got:  %q\n want: %q", gotQuery, tc.wantQuery)
			}

			if len(gotNamedParams) != len(tc.wantNamedParams) {
				t.Errorf("namedParams length\n got:  %d %v\n want: %d %v",
					len(gotNamedParams), gotNamedParams,
					len(tc.wantNamedParams), tc.wantNamedParams)
				return
			}

			for i := range tc.wantNamedParams {
				if gotNamedParams[i] != tc.wantNamedParams[i] {
					t.Errorf("namedParams[%d]\n got:  %q\n want: %q",
						i, gotNamedParams[i], tc.wantNamedParams[i])
				}
			}
		})
	}
}

// ─── reorderParameters ───────────────────────────────────────────────────────

func TestReorderParameters(t *testing.T) {
	a := newTestAdapter()

	tests := []struct {
		name        string
		params      map[string]any
		namedParams []string
		want        []any
		wantErr     bool
	}{
		{
			name:        "single param",
			params:      map[string]any{"id": 42},
			namedParams: []string{"id"},
			want:        []any{42},
		},
		{
			name:        "multiple params reordered",
			params:      map[string]any{"a": "alpha", "b": 2, "c": true},
			namedParams: []string{"c", "a", "b"},
			want:        []any{true, "alpha", 2},
		},
		{
			name:        "repeated param name in placeholders",
			params:      map[string]any{"val": "x"},
			namedParams: []string{"val", "val"},
			want:        []any{"x", "x"},
		},
		{
			name:        "nil value is preserved",
			params:      map[string]any{"col": nil},
			namedParams: []string{"col"},
			want:        []any{nil},
		},
		{
			name:        "empty namedParams returns nil slice",
			params:      map[string]any{"id": 1},
			namedParams: []string{},
			want:        nil,
		},
		{
			name:        "missing param returns error",
			params:      map[string]any{"a": 1},
			namedParams: []string{"a", "b"},
			wantErr:     true,
		},
		{
			name:        "completely missing param map",
			params:      map[string]any{},
			namedParams: []string{"id"},
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.reorderParameters(tc.params, tc.namedParams)

			if tc.wantErr {
				if err == nil {
					t.Errorf("reorderParameters() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("reorderParameters() unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("reorderParameters()\n got:  %v\n want: %v", got, tc.want)
			}
		})
	}
}
