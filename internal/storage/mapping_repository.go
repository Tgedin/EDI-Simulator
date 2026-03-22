package storage

import (
	"context"
	"database/sql"
	"time"
)

// TransformationMapping represents one registered format-conversion route.
type TransformationMapping struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SourceFormat string    `json:"source_format"`
	TargetFormat string    `json:"target_format"`
	Active       bool      `json:"active"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

// MappingRepository defines the interface for transformation mapping storage.
type MappingRepository interface {
	// ListActive returns all mappings where active = true.
	ListActive(ctx context.Context) ([]TransformationMapping, error)

	// GetByFormats returns the active mapping for the given source/target pair.
	// Returns ErrMappingNotFound when no active mapping exists.
	GetByFormats(ctx context.Context, srcFormat, tgtFormat string) (*TransformationMapping, error)

	// GetByID returns the mapping with the given ID (active or not).
	// Returns ErrMappingNotFound when not found.
	GetByID(ctx context.Context, id string) (*TransformationMapping, error)

	// Create inserts a new TransformationMapping and returns the persisted record.
	Create(ctx context.Context, m *TransformationMapping) (*TransformationMapping, error)

	// Update replaces the name, description, and active flag for an existing mapping.
	Update(ctx context.Context, m *TransformationMapping) (*TransformationMapping, error)

	// Delete soft-deletes the mapping (sets active = false).
	// Returns ErrMappingNotFound when not found.
	Delete(ctx context.Context, id string) error

	// Close closes the database connection.
	Close() error
}

// ---- Mock implementation (for tests) ----------------------------------------

// MockMappingRepository is an in-memory MappingRepository for testing.
type MockMappingRepository struct {
	Mappings []TransformationMapping
}

// NewMockMappingRepository creates a repository pre-seeded with the default
// four format-pair routes.
func NewMockMappingRepository() *MockMappingRepository {
	return &MockMappingRepository{
		Mappings: []TransformationMapping{
			{ID: "m1", Name: "X12 -> EDIFACT", SourceFormat: "x12", TargetFormat: "edifact", Active: true, Description: "Purchase Order: X12 to UN/EDIFACT"},
			{ID: "m2", Name: "EDIFACT -> X12", SourceFormat: "edifact", TargetFormat: "x12", Active: true, Description: "Purchase Order: UN/EDIFACT to X12"},
			{ID: "m3", Name: "X12 -> XML", SourceFormat: "x12", TargetFormat: "xml", Active: true, Description: "Purchase Order: X12 to Generic XML"},
			{ID: "m4", Name: "EDIFACT -> XML", SourceFormat: "edifact", TargetFormat: "xml", Active: true, Description: "Purchase Order: EDIFACT to Generic XML"},
		},
	}
}

// ListActive returns all active mappings.
func (m *MockMappingRepository) ListActive(_ context.Context) ([]TransformationMapping, error) {
	var result []TransformationMapping
	for _, mapping := range m.Mappings {
		if mapping.Active {
			result = append(result, mapping)
		}
	}
	return result, nil
}

// GetByFormats returns the first active mapping matching the given pair.
func (m *MockMappingRepository) GetByFormats(_ context.Context, src, tgt string) (*TransformationMapping, error) {
	for _, mapping := range m.Mappings {
		if mapping.Active && mapping.SourceFormat == src && mapping.TargetFormat == tgt {
			c := mapping
			return &c, nil
		}
	}
	return nil, ErrMappingNotFound
}

// GetByID returns the mapping with the given ID regardless of active status.
func (m *MockMappingRepository) GetByID(_ context.Context, id string) (*TransformationMapping, error) {
	for _, mapping := range m.Mappings {
		if mapping.ID == id {
			c := mapping
			return &c, nil
		}
	}
	return nil, ErrMappingNotFound
}

// Create inserts a new mapping and appends it to the in-memory slice.
func (m *MockMappingRepository) Create(_ context.Context, tm *TransformationMapping) (*TransformationMapping, error) {
	c := *tm
	if c.ID == "" {
		c.ID = "mock-" + tm.Name
	}
	c.Active = true
	m.Mappings = append(m.Mappings, c)
	return &c, nil
}

// Update replaces the editable fields for the matching mapping.
func (m *MockMappingRepository) Update(_ context.Context, tm *TransformationMapping) (*TransformationMapping, error) {
	for i, mapping := range m.Mappings {
		if mapping.ID == tm.ID {
			m.Mappings[i].Name = tm.Name
			m.Mappings[i].Description = tm.Description
			m.Mappings[i].Active = tm.Active
			updated := m.Mappings[i]
			return &updated, nil
		}
	}
	return nil, ErrMappingNotFound
}

// Delete soft-deletes the mapping by setting active = false.
func (m *MockMappingRepository) Delete(_ context.Context, id string) error {
	for i, mapping := range m.Mappings {
		if mapping.ID == id {
			m.Mappings[i].Active = false
			return nil
		}
	}
	return ErrMappingNotFound
}

// Close is a no-op for the mock.
func (m *MockMappingRepository) Close() error { return nil }

// ---- Postgres implementation -------------------------------------------------

// PostgresMappingRepository implements MappingRepository for PostgreSQL.
type PostgresMappingRepository struct {
	db *sql.DB
}

// NewPostgresMappingRepository creates a new PostgreSQL mapping repository.
func NewPostgresMappingRepository(db *sql.DB) *PostgresMappingRepository {
	return &PostgresMappingRepository{db: db}
}

// ListActive returns all active transformation mappings.
func (r *PostgresMappingRepository) ListActive(ctx context.Context) ([]TransformationMapping, error) {
	query := `
		SELECT id, name, source_format, target_format, active, COALESCE(description, ''), created_at
		FROM transformation_mappings
		WHERE active = true
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []TransformationMapping
	for rows.Next() {
		m := TransformationMapping{}
		if err := rows.Scan(
			&m.ID, &m.Name, &m.SourceFormat, &m.TargetFormat, &m.Active, &m.Description, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// GetByFormats returns the active mapping for the given source/target pair.
func (r *PostgresMappingRepository) GetByFormats(ctx context.Context, src, tgt string) (*TransformationMapping, error) {
	query := `
		SELECT id, name, source_format, target_format, active, COALESCE(description, ''), created_at
		FROM transformation_mappings
		WHERE active = true AND source_format = $1 AND target_format = $2
		LIMIT 1
	`
	m := &TransformationMapping{}
	err := r.db.QueryRowContext(ctx, query, src, tgt).Scan(
		&m.ID, &m.Name, &m.SourceFormat, &m.TargetFormat, &m.Active, &m.Description, &m.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrMappingNotFound
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Close closes the database connection.
func (r *PostgresMappingRepository) Close() error { return r.db.Close() }

const mappingSelectCols = `id, name, source_format, target_format, active, COALESCE(description, ''), created_at`

// GetByID returns a single mapping regardless of active status.
func (r *PostgresMappingRepository) GetByID(ctx context.Context, id string) (*TransformationMapping, error) {
	query := `SELECT ` + mappingSelectCols + ` FROM transformation_mappings WHERE id = $1`
	m := &TransformationMapping{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.Name, &m.SourceFormat, &m.TargetFormat, &m.Active, &m.Description, &m.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrMappingNotFound
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Create inserts a new TransformationMapping row and returns the full persisted record.
func (r *PostgresMappingRepository) Create(ctx context.Context, tm *TransformationMapping) (*TransformationMapping, error) {
	query := `
		INSERT INTO transformation_mappings (name, source_format, target_format, description, active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + mappingSelectCols
	m := &TransformationMapping{}
	err := r.db.QueryRowContext(ctx, query,
		tm.Name, tm.SourceFormat, tm.TargetFormat, tm.Description, true,
	).Scan(&m.ID, &m.Name, &m.SourceFormat, &m.TargetFormat, &m.Active, &m.Description, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Update replaces name, description, and active status for an existing mapping.
func (r *PostgresMappingRepository) Update(ctx context.Context, tm *TransformationMapping) (*TransformationMapping, error) {
	query := `
		UPDATE transformation_mappings
		SET name = $1, description = $2, active = $3
		WHERE id = $4
		RETURNING ` + mappingSelectCols
	m := &TransformationMapping{}
	err := r.db.QueryRowContext(ctx, query, tm.Name, tm.Description, tm.Active, tm.ID).Scan(
		&m.ID, &m.Name, &m.SourceFormat, &m.TargetFormat, &m.Active, &m.Description, &m.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrMappingNotFound
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Delete soft-deletes a mapping by setting active = false.
func (r *PostgresMappingRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE transformation_mappings SET active = false WHERE id = $1`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrMappingNotFound
	}
	return nil
}
