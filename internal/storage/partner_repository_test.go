package storage

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ---- Mock partner repository tests ------------------------------------------

func TestMockPartnerRepository_ListActive(t *testing.T) {
	repo := NewMockPartnerRepository()
	partners, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(partners) != 8 {
		t.Errorf("ListActive returned %d partners, want 8", len(partners))
	}
	for _, p := range partners {
		if !p.Active {
			t.Errorf("ListActive returned inactive partner %q", p.Name)
		}
	}
}

func TestMockPartnerRepository_ListActive_FiltersInactive(t *testing.T) {
	repo := NewMockPartnerRepository()
	repo.Partners[0].Active = false

	partners, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(partners) != 7 {
		t.Errorf("ListActive returned %d partners after deactivation, want 7", len(partners))
	}
}

func TestMockPartnerRepository_GetByID_Found(t *testing.T) {
	repo := NewMockPartnerRepository()
	p, err := repo.GetByID(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "Autoparts Co" {
		t.Errorf("expected 'Autoparts Co', got %q", p.Name)
	}
	if p.PreferredFormat != "x12" {
		t.Errorf("expected preferred_format 'x12', got %q", p.PreferredFormat)
	}
}

func TestMockPartnerRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMockPartnerRepository()
	_, err := repo.GetByID(context.Background(), "nonexistent")
	if err != ErrPartnerNotFound {
		t.Errorf("expected ErrPartnerNotFound, got %v", err)
	}
}

func TestMockPartnerRepository_AllFormatsRepresented(t *testing.T) {
	repo := NewMockPartnerRepository()
	partners, _ := repo.ListActive(context.Background())

	formats := make(map[string]bool)
	for _, p := range partners {
		formats[p.PreferredFormat] = true
	}
	for _, f := range []string{"x12", "edifact", "xml"} {
		if !formats[f] {
			t.Errorf("no active partner with preferred_format %q", f)
		}
	}
}

// ---- Postgres partner repository tests (sqlmock) ----------------------------

var partnerCols = []string{"id", "name", "country", "preferred_format", "edi_qualifier", "edi_id", "active", "created_at"}

func TestPostgresPartnerRepository_ListActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows(partnerCols).
		AddRow("uuid-1", "Autoparts Co", "US", "x12", "01", "AUTOPARTS01", true, testTime()).
		AddRow("uuid-2", "EuroParts GmbH", "DE", "edifact", "14", "EUROPARTS01", true, testTime())

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	repo := NewPostgresPartnerRepository(db)
	partners, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive error: %v", err)
	}
	if len(partners) != 2 {
		t.Errorf("expected 2 partners, got %d", len(partners))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}

func TestPostgresPartnerRepository_GetByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT`).
		WithArgs("uuid-1").
		WillReturnRows(sqlmock.NewRows(partnerCols).
			AddRow("uuid-1", "Autoparts Co", "US", "x12", "01", "AUTOPARTS01", true, testTime()))

	repo := NewPostgresPartnerRepository(db)
	p, err := repo.GetByID(context.Background(), "uuid-1")
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if p.Name != "Autoparts Co" {
		t.Errorf("expected 'Autoparts Co', got %q", p.Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}

func TestPostgresPartnerRepository_GetByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT`).
		WithArgs("no-such-id").
		WillReturnRows(sqlmock.NewRows(partnerCols))

	repo := NewPostgresPartnerRepository(db)
	_, err = repo.GetByID(context.Background(), "no-such-id")
	if err != ErrPartnerNotFound {
		t.Errorf("expected ErrPartnerNotFound, got %v", err)
	}
	if err2 := mock.ExpectationsWereMet(); err2 != nil {
		t.Errorf("sqlmock expectations not met: %v", err2)
	}
}

// ---- Mock mapping CRUD tests ------------------------------------------------

func TestMockMappingRepository_GetByID_Found(t *testing.T) {
	repo := NewMockMappingRepository()
	m, err := repo.GetByID(context.Background(), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "m1" {
		t.Errorf("expected id 'm1', got %q", m.ID)
	}
}

func TestMockMappingRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMockMappingRepository()
	_, err := repo.GetByID(context.Background(), "no-such-id")
	if err != ErrMappingNotFound {
		t.Errorf("expected ErrMappingNotFound, got %v", err)
	}
}

func TestMockMappingRepository_Create(t *testing.T) {
	repo := NewMockMappingRepository()
	before := len(repo.Mappings)

	created, err := repo.Create(context.Background(), &TransformationMapping{
		Name:         "JSON -> X12",
		SourceFormat: "json",
		TargetFormat: "x12",
		Description:  "Test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Name != "JSON -> X12" {
		t.Errorf("expected name 'JSON -> X12', got %q", created.Name)
	}
	if !created.Active {
		t.Error("created mapping should be active")
	}
	if len(repo.Mappings) != before+1 {
		t.Errorf("expected %d mappings after create, got %d", before+1, len(repo.Mappings))
	}
}

func TestMockMappingRepository_Update(t *testing.T) {
	repo := NewMockMappingRepository()
	original := repo.Mappings[0]

	updated, err := repo.Update(context.Background(), &TransformationMapping{
		ID:          original.ID,
		Name:        "Updated Name",
		Description: "New Description",
		Active:      original.Active,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("expected 'Updated Name', got %q", updated.Name)
	}
	if updated.Description != "New Description" {
		t.Errorf("expected 'New Description', got %q", updated.Description)
	}
}

func TestMockMappingRepository_Update_NotFound(t *testing.T) {
	repo := NewMockMappingRepository()
	_, err := repo.Update(context.Background(), &TransformationMapping{ID: "nonexistent", Name: "X"})
	if err != ErrMappingNotFound {
		t.Errorf("expected ErrMappingNotFound, got %v", err)
	}
}

func TestMockMappingRepository_Delete(t *testing.T) {
	repo := NewMockMappingRepository()

	if err := repo.Delete(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should no longer appear in active list
	actives, _ := repo.ListActive(context.Background())
	for _, m := range actives {
		if m.ID == "m1" {
			t.Error("deleted mapping still appears in active list")
		}
	}
}

func TestMockMappingRepository_Delete_NotFound(t *testing.T) {
	repo := NewMockMappingRepository()
	err := repo.Delete(context.Background(), "nonexistent")
	if err != ErrMappingNotFound {
		t.Errorf("expected ErrMappingNotFound, got %v", err)
	}
}

// ---- Postgres mapping CRUD tests (sqlmock) -----------------------------------

func TestPostgresMappingRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO transformation_mappings`).
		WithArgs("X12 -> JSON", "x12", "json", "test", true).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "source_format", "target_format", "active", "description", "created_at",
		}).AddRow("new-id", "X12 -> JSON", "x12", "json", true, "test", testTime()))

	repo := NewPostgresMappingRepository(db)
	created, err := repo.Create(context.Background(), &TransformationMapping{
		Name: "X12 -> JSON", SourceFormat: "x12", TargetFormat: "json", Description: "test",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID != "new-id" {
		t.Errorf("expected id 'new-id', got %q", created.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}

func TestPostgresMappingRepository_Delete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE transformation_mappings SET active = false`).
		WithArgs("some-id").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewPostgresMappingRepository(db)
	if err := repo.Delete(context.Background(), "some-id"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}

func TestPostgresMappingRepository_Delete_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE transformation_mappings SET active = false`).
		WithArgs("no-such-id").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewPostgresMappingRepository(db)
	err = repo.Delete(context.Background(), "no-such-id")
	if err != ErrMappingNotFound {
		t.Errorf("expected ErrMappingNotFound, got %v", err)
	}
	if err2 := mock.ExpectationsWereMet(); err2 != nil {
		t.Errorf("sqlmock expectations not met: %v", err2)
	}
}

// Compile-time implementation checks
var _ PartnerRepository = (*MockPartnerRepository)(nil)
var _ PartnerRepository = (*PostgresPartnerRepository)(nil)
