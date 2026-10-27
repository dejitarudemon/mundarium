package locations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

var (
	// ErrLocationWouldCreateCycle is returned when a parent update would form a cycle.
	ErrLocationWouldCreateCycle = errors.New("location would create a cycle")
)

// Location is a place in the world hierarchy (plane, region, room, and so on).
// A root location has a nil ParentID.
type Location struct {
	ID       repositories.ID  `gorm:"primaryKey" json:"id"`
	ParentID *repositories.ID `gorm:"index" json:"parent_id"` // parent location ID; nil for a root
	Parent   *Location        `gorm:"foreignKey:ParentID" json:"parent"`

	Type      LocationType     `gorm:"not null" json:"type"`
	SubtypeID *repositories.ID `gorm:"index" json:"subtype_id"`
	Subtype   *LocationSubtype `gorm:"foreignKey:SubtypeID" json:"subtype"`

	Name        string `gorm:"not null" json:"name"`
	Description string `gorm:"text" json:"description"`

	Details repositories.JSON `json:"Details"`
}

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
	Location
	RelDepth int `gorm:"column:rel_depth"`
}

// CreateLocation inserts a new location and returns the persisted record
// (including the generated ID and any DB defaults).
// Hierarchy validity (including cycles) is not checked here; use
// PatchLocationParent or PutLocation when reparenting an existing location.
func (r *LocationRepository) CreateLocation(ctx context.Context, location *Location) (*Location, error) {
	if err := r.db.WithContext(ctx).Create(location).Error; err != nil {
		r.error("failed to create location", "type", "LocationRepository", "func", "CreateLocation", "data", &location, "error", err)
		return nil, err
	}

	r.info("location is created", "type", "LocationRepository", "func", "CreateLocation", "data", &location)

	return location, nil
}

// GetLocation returns the location with the given id, preloading its Subtype.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetLocation(ctx context.Context, id repositories.ID) (*Location, error) {
	var location Location

	if err := r.db.
		WithContext(ctx).
		Where("id = ?", id).
		Preload("Subtype").
		First(&location).Error; err != nil {
		r.error("failed to get location", "type", "LocationRepository", "func", "GetLocation", "id", id, "error", err)
		return nil, err
	}

	r.debug("got location", "type", "LocationRepository", "func", "GetLocation", "data", &location)

	return &location, nil
}

// FindChildLocations returns descendants of id grouped by depth.
//
// Depth semantics:
//   - 0 or less: empty map
//   - 1: direct children only (map key 1)
//   - 2 or more: children, grandchildren, … up to the given depth (keys 1..depth)
//
// If id does not exist, gorm.ErrRecordNotFound is returned.
// Each level is loaded with one query ordered by id. Already seen IDs are
// skipped to break parent_id cycles. When loadSubtypes is true, each level
// preloads Subtype.
func (r *LocationRepository) FindChildLocations(ctx context.Context, id repositories.ID, depth int, loadSubtypes bool) (map[int][]Location, error) {
	result := make(map[int][]Location)
	if depth <= 0 {
		r.error("failed to find location", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "error", fmt.Sprintf("invalid depth %v", depth))
		return result, nil
	}

	var exists int64
	if err := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Count(&exists).Error; err != nil {
		r.error("failed to find location", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "error", err)
		return nil, err
	}
	if exists == 0 {
		r.error("failed to find location", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "error", gorm.ErrRecordNotFound)
		return nil, gorm.ErrRecordNotFound
	}

	seen := map[repositories.ID]struct{}{id: {}}
	parentIDs := []repositories.ID{id}

	for d := 1; d <= depth; d++ {
		if len(parentIDs) == 0 {
			break
		}

		var found []Location
		q := r.db.WithContext(ctx).
			Where("parent_id IN ?", parentIDs).
			Order("id")

		if loadSubtypes {
			q = q.Preload("Subtype")
		}

		if err := q.Find(&found).Error; err != nil {
			r.error("failed to find location children", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "level", d, "depth", depth, "error", err)
			return nil, err
		}

		parentIDs = parentIDs[:0]
		level := make([]Location, 0, len(found))
		for _, loc := range found {
			if _, ok := seen[loc.ID]; ok {
				continue
			}
			seen[loc.ID] = struct{}{}
			level = append(level, loc)
			parentIDs = append(parentIDs, loc.ID)
		}

		if len(level) == 0 {
			break
		}
		result[d] = level
	}

	r.debug("found location", "type", "LocationRepository", "func", "FindChildLocations", "data", result)
	return result, nil
}

// FindLocations returns the scope location plus descendants matching filter.
// The scope is always included when it exists; filters apply only to descendants.
// Depth 0 returns only the scope; larger depths include descendants up to that relative depth.
// Traversal uses a recursive CTE so non-matching intermediates still expand the tree;
// already-seen IDs are excluded via a path array to break parent_id cycles.
// Returns gorm.ErrRecordNotFound if ScopeID does not exist.
// Returns an error if Depth is outside 0..MaxDepth().
func (r *LocationRepository) FindLocations(ctx context.Context, filter FindLocationsFilter) (FindLocationsResult, error) {
	if filter.Depth < 0 || filter.Depth > r.maxDepth {
		err := errors.New("depth out of range")
		r.error("failed to find locations", "type", "LocationRepository", "func", "FindLocations", "scope_id", filter.ScopeID, "depth", filter.Depth, "error", err)
		return FindLocationsResult{}, err
	}

	matchSQL, matchArgs := findLocationsMatch(filter)
	args := append([]any{filter.ScopeID, filter.Depth}, matchArgs...)

	rows, err := r.scanFindLocations(ctx, matchSQL, args)
	if err != nil {
		r.error("failed to find locations", "type", "LocationRepository", "func", "FindLocations", "scope_id", filter.ScopeID, "depth", filter.Depth, "error", err)
		return FindLocationsResult{}, err
	}
	if len(rows) == 0 {
		r.error("failed to find locations", "type", "LocationRepository", "func", "FindLocations", "scope_id", filter.ScopeID, "error", gorm.ErrRecordNotFound)
		return FindLocationsResult{}, gorm.ErrRecordNotFound
	}

	result := groupFindLocations(rows)
	r.debug("found locations", "type", "LocationRepository", "func", "FindLocations", "scope_id", filter.ScopeID, "children_levels", len(result.Children))
	return result, nil
}

// findLocationsMatch builds the descendant WHERE fragment and its bind args.
// The scope row (rel_depth = 0) is never filtered by this clause.
func findLocationsMatch(filter FindLocationsFilter) (string, []any) {
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

func groupFindLocations(rows []findLocationsRow) FindLocationsResult {
	result := FindLocationsResult{
		Scope:    rows[0].Location,
		Children: make(map[int][]Location),
	}
	for _, item := range rows[1:] {
		result.Children[item.RelDepth] = append(result.Children[item.RelDepth], item.Location)
	}
	return result
}

// PutLocation updates mutable fields of an existing location identified by location.ID.
//
// Before writing, checkCircle walks ancestors of location.ParentID (no transaction)
// and rejects the update if that parent link would create a cycle.
// A nil ParentID means the location becomes a root.
//
// Returns ErrLocationWouldCreateCycle if the new parent would form a cycle.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutLocation(ctx context.Context, location *Location) error {
	cycle, err := r.checkCircle(ctx, location.ID, location.ParentID)
	if err != nil {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", err)
		return err
	}
	if cycle {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", ErrLocationWouldCreateCycle)
		return ErrLocationWouldCreateCycle
	}

	result := r.db.WithContext(ctx).
		Model(&Location{}).
		Where("id = ?", location.ID).
		Select(
			"ParentID",
			"Type",
			"SubtypeID",
			"Name",
			"Description",
			"Details",
		).
		Updates(location)

	if result.Error != nil {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location is updated", "type", "LocationRepository", "func", "PutLocation", "data", location)
	return nil
}

// PatchLocationParent sets the parent of the location identified by id to parentID.
//
// Before writing, checkCircle walks ancestors of parentID (no transaction)
// and rejects the change if that parent link would create a cycle.
// A nil parentID means the location becomes a root.
//
// Returns ErrLocationWouldCreateCycle if the new parent would form a cycle.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationParent(ctx context.Context, id repositories.ID, parentID *repositories.ID) error {
	cycle, err := r.checkCircle(ctx, id, parentID)
	if err != nil {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", err)
		return err
	}
	if cycle {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", ErrLocationWouldCreateCycle)
		return ErrLocationWouldCreateCycle
	}

	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("parent_id", parentID)

	if err := query.Error; err != nil {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location parent is patched", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "parent_id", parentID)
	return nil
}

// PatchLocationName sets the name of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationName(ctx context.Context, id repositories.ID, name string) error {
	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("name", name)

	if err := query.Error; err != nil {
		r.error("failed to patch location name", "type", "LocationRepository", "func", "PatchLocationName", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location name", "type", "LocationRepository", "func", "PatchLocationName", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location name is patched", "type", "LocationRepository", "func", "PatchLocationName", "id", id, "name", name)
	return nil
}

// PatchLocationDetails sets the details of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationDetails(ctx context.Context, id repositories.ID, details repositories.JSON) error {
	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("details", details)

	if err := query.Error; err != nil {
		r.error("failed to patch location details", "type", "LocationRepository", "func", "PatchLocationDetails", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location details", "type", "LocationRepository", "func", "PatchLocationDetails", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location details are patched", "type", "LocationRepository", "func", "PatchLocationDetails", "id", id)
	return nil
}

// PatchLocationDescription sets the description of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationDescription(ctx context.Context, id repositories.ID, description string) error {
	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("description", description)

	if err := query.Error; err != nil {
		r.error("failed to patch location description", "type", "LocationRepository", "func", "PatchLocationDescription", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location description", "type", "LocationRepository", "func", "PatchLocationDescription", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location description is patched", "type", "LocationRepository", "func", "PatchLocationDescription", "id", id)
	return nil
}

// checkCircle reports whether setting id's parent to newParent would create a cycle.
//
// It reads the current ancestor chain of newParent and does not write.
// Returns true if a cycle would be created, false otherwise.
// A nil newParent means becoming a root and is never a cycle.
// A nil parent_id on an ancestor ends the walk as a tree root.
func (r *LocationRepository) checkCircle(ctx context.Context, id repositories.ID, newParent *repositories.ID) (bool, error) {
	if newParent == nil {
		return false, nil
	}
	if *newParent == 0 {
		return false, errors.New("parent id is zero")
	}
	if *newParent == id {
		return true, nil
	}

	next := *newParent
	seen := map[repositories.ID]struct{}{
		id:   {},
		next: {},
	}

	for ctx.Err() == nil {
		var parentID *repositories.ID
		if err := r.db.WithContext(ctx).
			Model(&Location{}).
			Select("parent_id").
			Where("id = ?", next).
			Scan(&parentID).Error; err != nil {
			r.error("failed to check location cycle", "type", "LocationRepository", "func", "checkCircle", "id", id, "next", next, "error", err)
			return false, err
		}

		// root: parent is null
		if parentID == nil {
			return false, nil
		}

		next = *parentID
		if next == id {
			return true, nil
		}

		if _, ok := seen[next]; ok {
			return true, nil
		}

		seen[next] = struct{}{}
	}

	return false, ctx.Err()
}
