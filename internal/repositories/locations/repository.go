// Package locations provides data-access helpers for location domain models.
package locations

import (
	"context"
	"errors"

	liblocations "github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

// LocationRepository persists and queries Location, LocationSubtype, and Connection records.
type LocationRepository struct {
	db       *gorm.DB
	maxDepth int
}

// NewLocationRepository creates a LocationRepository backed by db.
// maxDepth is the maximum relative depth accepted by FindLocations (inclusive).
// Returns an error if db is nil or maxDepth is negative.
func NewLocationRepository(db *gorm.DB, maxDepth int) (*LocationRepository, error) {
	if db == nil {
		return nil, errors.New("expected *gorm.DB, got nil")
	}
	if maxDepth < 0 {
		return nil, errors.New("max depth must be >= 0")
	}

	return &LocationRepository{db: db, maxDepth: maxDepth}, nil
}

// MaxDepth returns the maximum relative depth accepted by FindLocations.
func (r *LocationRepository) MaxDepth() int {
	return r.maxDepth
}

// CreateLocation inserts a new location and returns the persisted record
// (including the generated ID and any DB defaults).
// Hierarchy validity (including cycles) is not checked here; use
// PatchLocationParent or PutLocation when reparenting an existing location.
func (r *LocationRepository) CreateLocation(ctx context.Context, location *liblocations.Location) (*liblocations.Location, error) {
	if err := r.db.WithContext(ctx).Create(location).Error; err != nil {
		return nil, err
	}

	return location, nil
}

// GetLocation returns the location with the given id, preloading its Subtype.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetLocation(ctx context.Context, id types.ID) (*liblocations.Location, error) {
	var location liblocations.Location

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
func (r *LocationRepository) FindChildLocations(ctx context.Context, id types.ID, depth int, loadSubtypes bool) (map[int][]liblocations.Location, error) {
	result := make(map[int][]liblocations.Location)
	if depth <= 0 {
		return result, nil
	}

	var exists int64
	if err := r.db.WithContext(ctx).Model(&liblocations.Location{}).Where("id = ?", id).Count(&exists).Error; err != nil {
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

		var found []liblocations.Location
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
		level := make([]liblocations.Location, 0, len(found))
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

// PutLocation updates mutable fields of an existing location identified by location.ID.
//
// Before writing, checkCircle walks ancestors of location.ParentID (no transaction)
// and rejects the update if that parent link would create a cycle.
// A nil ParentID means the location becomes a root.
//
// Returns an error with message "location would create a cycle" if the new
// parent would form a cycle. Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutLocation(ctx context.Context, location *liblocations.Location) error {
	cycle, err := r.checkCircle(ctx, location.ID, location.ParentID)
	if err != nil {
		return err
	}
	if cycle {
		return errors.New("location would create a cycle")
	}

	result := r.db.WithContext(ctx).
		Model(&liblocations.Location{}).
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

	query := r.db.WithContext(ctx).Model(&liblocations.Location{}).Where("id = ?", id).Update("parent_id", parentID)

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
	query := r.db.WithContext(ctx).Model(&liblocations.Location{}).Where("id = ?", id).Update("name", name)

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
	query := r.db.WithContext(ctx).Model(&liblocations.Location{}).Where("id = ?", id).Update("details", details)

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
	query := r.db.WithContext(ctx).Model(&liblocations.Location{}).Where("id = ?", id).Update("description", description)

	if err := query.Error; err != nil {
		return err
	}

	if query.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// CreateSubtype inserts a new location subtype.
func (r *LocationRepository) CreateSubtype(ctx context.Context, subtype *liblocations.LocationSubtype) error {
	return r.db.WithContext(ctx).Create(subtype).Error
}

// GetSubtype returns the location subtype with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetSubtype(ctx context.Context, id types.ID) (*liblocations.LocationSubtype, error) {
	var subtype liblocations.LocationSubtype

	if err := r.db.
		WithContext(ctx).
		First(&subtype, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &subtype, nil
}

// FindSubtypes returns subtypes matching any of the given keys and/or location types.
// Empty keys or types means that filter is not applied. If both are empty, all subtypes are returned.
func (r *LocationRepository) FindSubtypes(ctx context.Context, keys []string, types []types.LocationType) ([]liblocations.LocationSubtype, error) {
	var subtypes []liblocations.LocationSubtype

	query := r.db.WithContext(ctx).Model(liblocations.LocationSubtype{})

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
func (r *LocationRepository) CreateConnection(ctx context.Context, connection *liblocations.Connection) error {
	return r.db.WithContext(ctx).Create(connection).Error
}

// GetConnection returns the connection with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetConnection(ctx context.Context, id types.ID) (*liblocations.Connection, error) {
	var connection liblocations.Connection

	if err := r.db.WithContext(ctx).First(&connection, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &connection, nil
}

// FindInderectConnections returns connections directed from fromID to toID.
func (r *LocationRepository) FindInderectConnections(ctx context.Context, fromID, toID types.ID) ([]liblocations.Connection, error) {
	connections := make([]liblocations.Connection, 0)

	if err := r.db.WithContext(ctx).Find(&connections, "from_id = ? AND to_id = ?", fromID, toID).Error; err != nil {
		return nil, err
	}

	return connections, nil
}

// PutConnection updates mutable fields of an existing connection identified by connection.ID.
// Returns an error if FromID and ToID refer to the same location.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutConnection(ctx context.Context, connection *liblocations.Connection) error {
	if connection.FromID == connection.ToID {
		return errors.New("connection between the same locations")
	}

	result := r.db.WithContext(ctx).
		Model(&liblocations.Connection{}).
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
	return r.db.WithContext(ctx).Delete(liblocations.Connection{}, "id = ?", id).Error
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
			Model(&liblocations.Location{}).
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
