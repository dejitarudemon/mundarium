// Package repositories provides data-access helpers for domain models.
package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

// FindLocationsFilter selects locations under ScopeID with optional filters.
//
// Depth semantics (relative to ScopeID):
//   - 0: only the scope location
//   - 1: scope + direct children
//   - n: scope + descendants up to relative depth n (max 5)
type FindLocationsFilter struct {
	ScopeID   types.ID
	Query     *string
	Type      *types.LocationType
	SubtypeID *types.ID
	Depth     int
}

// FindLocationsResult is the scope plus matching descendants grouped by relative depth.
// Depth keys: 1 = direct children, 2 = grandchildren, and so on.
type FindLocationsResult struct {
	Scope    locations.Location
	Children map[int][]locations.Location
}

// LocationRepository persists and queries Location, LocationSubtype, and Connection records.
type LocationRepository struct {
	db *gorm.DB
}

// NewLocationRepository creates a LocationRepository backed by db.
// Returns an error if db is nil.
func NewLocationRepository(db *gorm.DB) (*LocationRepository, error) {
	if db == nil {
		return nil, errors.New("expected *gorm.DB, got nil")
	}

	return &LocationRepository{db: db}, nil
}

// CreateLocation inserts a new location and returns the persisted record
// (including the generated ID and any DB defaults).
// Hierarchy validity (including cycles) is not checked here; use
// PatchLocationParent or PutLocation when reparenting an existing location.
func (r *LocationRepository) CreateLocation(ctx context.Context, location *locations.Location) (*locations.Location, error) {
	if err := r.db.WithContext(ctx).Create(location).Error; err != nil {
		return nil, err
	}

	return location, nil
}

// GetLocation returns the location with the given id, preloading its Subtype.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetLocation(ctx context.Context, id types.ID) (*locations.Location, error) {
	var location locations.Location

	if err := r.db.
		WithContext(ctx).
		Where("id = ?", id).
		Preload("Subtype").
		First(&location).Error; err != nil {
		return nil, err
	}

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
func (r *LocationRepository) FindChildLocations(ctx context.Context, id types.ID, depth int, loadSubtypes bool) (map[int][]locations.Location, error) {
	result := make(map[int][]locations.Location)
	if depth <= 0 {
		return result, nil
	}

	var exists int64
	if err := r.db.WithContext(ctx).Model(&locations.Location{}).Where("id = ?", id).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	seen := map[types.ID]struct{}{id: {}}
	parentIDs := []types.ID{id}

	for d := 1; d <= depth; d++ {
		if len(parentIDs) == 0 {
			break
		}

		var found []locations.Location
		q := r.db.WithContext(ctx).
			Where("parent_id IN ?", parentIDs).
			Order("id")

		if loadSubtypes {
			q = q.Preload("Subtype")
		}

		if err := q.Find(&found).Error; err != nil {
			return nil, err
		}

		parentIDs = parentIDs[:0]
		level := make([]locations.Location, 0, len(found))
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

	return result, nil
}

// FindLocations returns the scope location plus descendants matching filter.
// The scope is always included when it exists; filters apply only to descendants.
// Depth 0 returns only the scope; larger depths include descendants up to that relative depth.
// Traversal uses a recursive CTE so non-matching intermediates still expand the tree;
// already-seen IDs are excluded via a path array to break parent_id cycles.
// Returns gorm.ErrRecordNotFound if ScopeID does not exist.
// Returns an error if Depth is outside 0..5.
func (r *LocationRepository) FindLocations(ctx context.Context, filter FindLocationsFilter) (FindLocationsResult, error) {
	if filter.Depth < 0 || filter.Depth > 5 {
		return FindLocationsResult{}, errors.New("depth out of range")
	}

	args := []any{filter.ScopeID, filter.Depth}

	var match strings.Builder
	match.WriteString("TRUE")
	if filter.Type != nil {
		match.WriteString(" AND type = ?")
		args = append(args, *filter.Type)
	}
	if filter.SubtypeID != nil {
		match.WriteString(" AND subtype_id = ?")
		args = append(args, *filter.SubtypeID)
	}
	if filter.Query != nil && *filter.Query != "" {
		match.WriteString(" AND (name ILIKE '%' || ? || '%' OR description ILIKE '%' || ? || '%')")
		args = append(args, *filter.Query, *filter.Query)
	}

	const query = `
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

	type row struct {
		locations.Location
		RelDepth int `gorm:"column:rel_depth"`
	}

	var rows []row
	if err := r.db.WithContext(ctx).Raw(fmt.Sprintf(query, match.String()), args...).Scan(&rows).Error; err != nil {
		return FindLocationsResult{}, err
	}
	if len(rows) == 0 {
		return FindLocationsResult{}, gorm.ErrRecordNotFound
	}

	result := FindLocationsResult{
		Scope:    rows[0].Location,
		Children: make(map[int][]locations.Location),
	}
	for _, item := range rows[1:] {
		result.Children[item.RelDepth] = append(result.Children[item.RelDepth], item.Location)
	}
	return result, nil
}

// PutLocation updates mutable fields of an existing location identified by location.ID.
//
// Before writing, checkCircle walks ancestors of location.ParentID (no transaction)
// and rejects the update if that parent link would create a cycle.
// A nil ParentID means the location becomes a root.
//
// Returns an error with message "location would create a cycle" if the new
// parent would form a cycle. Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutLocation(ctx context.Context, location *locations.Location) error {
	cycle, err := r.checkCircle(ctx, location.ID, location.ParentID)
	if err != nil {
		return err
	}
	if cycle {
		return errors.New("location would create a cycle")
	}

	result := r.db.WithContext(ctx).
		Model(&locations.Location{}).
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
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// PatchLocationParent sets the parent of the location identified by id to parentID.
//
// Before writing, checkCircle walks ancestors of parentID (no transaction)
// and rejects the change if that parent link would create a cycle.
// A nil parentID means the location becomes a root.
//
// Returns an error with message "location would create a cycle" if the new
// parent would form a cycle. Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationParent(ctx context.Context, id types.ID, parentID *types.ID) error {
	cycle, err := r.checkCircle(ctx, id, parentID)
	if err != nil {
		return err
	}
	if cycle {
		return errors.New("location would create a cycle")
	}

	query := r.db.WithContext(ctx).Model(&locations.Location{}).Where("id = ?", id).Update("parent_id", parentID)

	if err := query.Error; err != nil {
		return err
	}

	if query.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// PatchLocationName sets the name of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationName(ctx context.Context, id types.ID, name string) error {
	query := r.db.WithContext(ctx).Model(&locations.Location{}).Where("id = ?", id).Update("name", name)

	if err := query.Error; err != nil {
		return err
	}

	if query.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// PatchLocationDetails sets the details of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationDetails(ctx context.Context, id types.ID, details types.JSON) error {
	query := r.db.WithContext(ctx).Model(&locations.Location{}).Where("id = ?", id).Update("details", details)

	if err := query.Error; err != nil {
		return err
	}

	if query.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// PatchLocationDescription sets the description of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationDescription(ctx context.Context, id types.ID, description string) error {
	query := r.db.WithContext(ctx).Model(&locations.Location{}).Where("id = ?", id).Update("description", description)

	if err := query.Error; err != nil {
		return err
	}

	if query.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// CreateSubtype inserts a new location subtype.
func (r *LocationRepository) CreateSubtype(ctx context.Context, subtype *locations.LocationSubtype) error {
	return r.db.WithContext(ctx).Create(subtype).Error
}

// GetSubtype returns the location subtype with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetSubtype(ctx context.Context, id types.ID) (*locations.LocationSubtype, error) {
	var subtype locations.LocationSubtype

	if err := r.db.
		WithContext(ctx).
		First(&subtype, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &subtype, nil
}

// FindSubtypes returns subtypes matching any of the given keys and/or location types.
// Empty keys or types means that filter is not applied. If both are empty, all subtypes are returned.
func (r *LocationRepository) FindSubtypes(ctx context.Context, keys []string, types []types.LocationType) ([]locations.LocationSubtype, error) {
	var subtypes []locations.LocationSubtype

	query := r.db.WithContext(ctx).Model(locations.LocationSubtype{})

	if len(keys) > 0 {
		query = query.Where("key IN ?", keys)
	}
	if len(types) > 0 {
		query = query.Where("type IN ?", types)
	}

	if err := query.Find(&subtypes).Error; err != nil {
		return nil, err
	}

	return subtypes, nil
}

// CreateConnection inserts a new connection between two locations.
// The repository does not validate whether the link is logically meaningful;
// that is left to the caller.
func (r *LocationRepository) CreateConnection(ctx context.Context, connection *locations.Connection) error {
	return r.db.WithContext(ctx).Create(connection).Error
}

// GetConnection returns the connection with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetConnection(ctx context.Context, id types.ID) (*locations.Connection, error) {
	var connection locations.Connection

	if err := r.db.WithContext(ctx).First(&connection, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &connection, nil
}

// FindInderectConnections returns connections directed from fromID to toID.
func (r *LocationRepository) FindInderectConnections(ctx context.Context, fromID, toID types.ID) ([]locations.Connection, error) {
	connections := make([]locations.Connection, 0)

	if err := r.db.WithContext(ctx).Find(&connections, "from_id = ? AND to_id = ?", fromID, toID).Error; err != nil {
		return nil, err
	}

	return connections, nil
}

// PutConnection updates mutable fields of an existing connection identified by connection.ID.
// Returns an error if FromID and ToID refer to the same location.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutConnection(ctx context.Context, connection *locations.Connection) error {
	if connection.FromID == connection.ToID {
		return errors.New("connection between the same locations")
	}

	result := r.db.WithContext(ctx).
		Model(&locations.Connection{}).
		Where("id = ?", connection.ID).
		Select(
			"FromID",
			"ToID",
			"Bidirectional",
			"State",
			"Details",
			"Description",
			"ExpiresAt",
		).
		Updates(connection)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// DeleteConnection deletes the connection with the given id.
func (r *LocationRepository) DeleteConnection(ctx context.Context, id types.ID) error {
	return r.db.WithContext(ctx).Delete(locations.Connection{}, "id = ?", id).Error
}

// checkCircle reports whether setting id's parent to newParent would create a cycle.
//
// It reads the current ancestor chain of newParent and does not write.
// Returns true if a cycle would be created, false otherwise.
// A nil newParent means becoming a root and is never a cycle.
// A nil parent_id on an ancestor ends the walk as a tree root.
func (r *LocationRepository) checkCircle(ctx context.Context, id types.ID, newParent *types.ID) (bool, error) {
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
	seen := map[types.ID]struct{}{
		id:   {},
		next: {},
	}

	for ctx.Err() == nil {
		var parentID *types.ID
		if err := r.db.WithContext(ctx).
			Model(&locations.Location{}).
			Select("parent_id").
			Where("id = ?", next).
			Scan(&parentID).Error; err != nil {
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
