package storage

import (
	"context"
	"database/sql"

	"github.com/theo-gedin/edi-simulator/internal/models"
)

// PartnerRepository defines the interface for trading partner storage.
type PartnerRepository interface {
	// ListActive returns all partners where active = true.
	ListActive(ctx context.Context) ([]models.TradingPartner, error)

	// GetByID returns a single partner by primary key.
	// Returns ErrPartnerNotFound when not found.
	GetByID(ctx context.Context, id string) (*models.TradingPartner, error)

	// Close closes the database connection.
	Close() error
}

// ---- Mock implementation ------------------------------------------------

// MockPartnerRepository is an in-memory PartnerRepository for testing.
type MockPartnerRepository struct {
	Partners []models.TradingPartner
}

// NewMockPartnerRepository returns a repository pre-seeded with the 8 default companies.
func NewMockPartnerRepository() *MockPartnerRepository {
	return &MockPartnerRepository{Partners: defaultPartners()}
}

func (m *MockPartnerRepository) ListActive(_ context.Context) ([]models.TradingPartner, error) {
	var result []models.TradingPartner
	for _, p := range m.Partners {
		if p.Active {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *MockPartnerRepository) GetByID(_ context.Context, id string) (*models.TradingPartner, error) {
	for _, p := range m.Partners {
		if p.ID == id {
			c := p
			return &c, nil
		}
	}
	return nil, ErrPartnerNotFound
}

func (m *MockPartnerRepository) Close() error { return nil }

// ---- Postgres implementation --------------------------------------------

// PostgresPartnerRepository implements PartnerRepository for PostgreSQL.
type PostgresPartnerRepository struct {
	db *sql.DB
}

// NewPostgresPartnerRepository creates a new PostgreSQL partner repository.
func NewPostgresPartnerRepository(db *sql.DB) *PostgresPartnerRepository {
	return &PostgresPartnerRepository{db: db}
}

const partnerSelectCols = `id, name, country, preferred_format, edi_qualifier, edi_id, active, created_at`

// ListActive returns all active trading partners ordered by name.
func (r *PostgresPartnerRepository) ListActive(ctx context.Context) ([]models.TradingPartner, error) {
	query := `SELECT ` + partnerSelectCols + `
		FROM trading_partners
		WHERE active = true
		ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []models.TradingPartner
	for rows.Next() {
		p := models.TradingPartner{}
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Country, &p.PreferredFormat,
			&p.EDIQualifier, &p.EDIID, &p.Active, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, p)
	}
	return partners, rows.Err()
}

// GetByID returns a single trading partner by its UUID.
func (r *PostgresPartnerRepository) GetByID(ctx context.Context, id string) (*models.TradingPartner, error) {
	query := `SELECT ` + partnerSelectCols + `
		FROM trading_partners WHERE id = $1`

	p := &models.TradingPartner{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Country, &p.PreferredFormat,
		&p.EDIQualifier, &p.EDIID, &p.Active, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPartnerNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Close closes the database connection.
func (r *PostgresPartnerRepository) Close() error { return r.db.Close() }

// defaultPartners returns the canonical list of 8 simulated companies.
// Used by the mock and by the seed migration.
func defaultPartners() []models.TradingPartner {
	return []models.TradingPartner{
		{ID: "p1", Name: "Autoparts Co", Country: "US", PreferredFormat: "x12", EDIQualifier: "01", EDIID: "AUTOPARTS01", Active: true},
		{ID: "p2", Name: "Supplier Corp", Country: "US", PreferredFormat: "x12", EDIQualifier: "01", EDIID: "SUPPLIER01", Active: true},
		{ID: "p3", Name: "EuroParts GmbH", Country: "DE", PreferredFormat: "edifact", EDIQualifier: "14", EDIID: "EUROPARTS01", Active: true},
		{ID: "p4", Name: "RetailEU SARL", Country: "FR", PreferredFormat: "edifact", EDIQualifier: "14", EDIID: "RETAILEU01", Active: true},
		{ID: "p5", Name: "AsiaPac Motors", Country: "HK", PreferredFormat: "xml", EDIQualifier: "ZZ", EDIID: "ASIAPAC01", Active: true},
		{ID: "p6", Name: "Global Auto Supply", Country: "US", PreferredFormat: "xml", EDIQualifier: "ZZ", EDIID: "GLOBALAUTO01", Active: true},
		{ID: "p7", Name: "NordikParts AS", Country: "NO", PreferredFormat: "edifact", EDIQualifier: "14", EDIID: "NORDIKP01", Active: true},
		{ID: "p8", Name: "Pacific Logistics Co", Country: "AU", PreferredFormat: "x12", EDIQualifier: "01", EDIID: "PACLOGIS01", Active: true},
	}
}
