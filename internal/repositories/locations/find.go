package locations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	filterslocations "github.com/dejitarudemon/mundarium/internal/libs/filters/locations"
	liblocations "github.com/dejitarudemon/mundarium/internal/libs/locations"
	"gorm.io/gorm"
)

const findLocationsCTE = `
WITH RECURSIVE tree AS (
	SELECT id, parent_id, type, subtype_id, name, description, details,
		0 AS rel_depth, ARRAY[id] AS path
	FROM locations
	WHERE id = ?
	UNION ALL
	SELECT c.id, c.parent_id, c.type, c.subtype_id, c.name, c.description, c.details,
		t.rel_depth + 1, t.path || c.id
	FROM locations c
	INNER JOIN tree t ON c.parent_id = t.id
	WHERE t.rel_depth < ?
		AND NOT (c.id = ANY(t.path))
)
SELECT id, parent_id, type, subtype_id, name, description, details, rel_depth
FROM tree
WHERE rel_depth = 0 OR (%s)
ORDER BY rel_depth, id`

type findLocationsRow struct {
	liblocations.Location
	RelDepth int `gorm:"column:rel_depth"`
}

// FindLocations returns the scope location plus descendants matching filter.
// The scope is always included when it exists; filters apply only to descendants.
// Depth 0 returns only the scope; larger depths include descendants up to that relative depth.
// Traversal uses a recursive CTE so non-matching intermediates still expand the tree;
// already-seen IDs are excluded via a path array to break parent_id cycles.
// Returns gorm.ErrRecordNotFound if ScopeID does not exist.
// Returns an error if Depth is outside 0..MaxDepth().
func (r *LocationRepository) FindLocations(ctx context.Context, filter filterslocations.FindLocationsFilter) (filterslocations.FindLocationsResult, error) {
	if filter.Depth < 0 || filter.Depth > r.maxDepth {
		return filterslocations.FindLocationsResult{}, errors.New("depth out of range")
	}

	matchSQL, matchArgs := findLocationsMatch(filter)
	args := append([]any{filter.ScopeID, filter.Depth}, matchArgs...)

	rows, err := r.scanFindLocations(ctx, matchSQL, args)
	if err != nil {
		return filterslocations.FindLocationsResult{}, err
	}
	if len(rows) == 0 {
		return filterslocations.FindLocationsResult{}, gorm.ErrRecordNotFound
	}

	return groupFindLocations(rows), nil
}

// findLocationsMatch builds the descendant WHERE fragment and its bind args.
// The scope row (rel_depth = 0) is never filtered by this clause.
func findLocationsMatch(filter filterslocations.FindLocationsFilter) (string, []any) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString("TRUE")

	if filter.Type != nil {
		b.WriteString(" AND type = ?")
		args = append(args, *filter.Type)
	}
	if filter.SubtypeID != nil {
		b.WriteString(" AND subtype_id = ?")
		args = append(args, *filter.SubtypeID)
	}
	if filter.Query != nil && *filter.Query != "" {
		b.WriteString(" AND (name ILIKE '%' || ? || '%' OR description ILIKE '%' || ? || '%')")
		args = append(args, *filter.Query, *filter.Query)
	}

	return b.String(), args
}

func (r *LocationRepository) scanFindLocations(ctx context.Context, matchSQL string, args []any) ([]findLocationsRow, error) {
	var rows []findLocationsRow
	err := r.db.WithContext(ctx).Raw(fmt.Sprintf(findLocationsCTE, matchSQL), args...).Scan(&rows).Error
	return rows, err
}

func groupFindLocations(rows []findLocationsRow) filterslocations.FindLocationsResult {
	result := filterslocations.FindLocationsResult{
		Scope:    rows[0].Location,
		Children: make(map[int][]liblocations.Location),
	}
	for _, item := range rows[1:] {
		result.Children[item.RelDepth] = append(result.Children[item.RelDepth], item.Location)
	}
	return result
}
