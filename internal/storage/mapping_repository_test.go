package storage

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func testTime() time.Time { return time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC) }

// ---- Mock mapping repository tests ------------------------------------------

func TestMockMappingRepository_ListActive(t *testing.T) {
	repo := NewMockMappingRepository()
	mappings, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mappings) != 4 {
		t.Errorf("ListActive returned %d mappings, want 4", len(mappings))
	}
	for _, m := range mappings {
		if !m.Active {
			t.Errorf("ListActive returned inactive mapping %q", m.Name)
		}
	}
}

func TestMockMappingRepository_ListActive_FiltersInactive(t *testing.T) {
	repo := NewMockMappingRepository()
	// Deactivate one mapping
	repo.Mappings[0].Active = false

	mappings, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mappings) != 3 {
		t.Errorf("ListActive returned %d mappings after deactivation, want 3", len(mappings))
	}
}

func TestMockMappingRepository_GetByFormats_Found(t *testing.T) {
	repo := NewMockMappingRepository()
	m, err := repo.GetByFormats(context.Background(), "x12", "edifact")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.SourceFormat != "x12" || m.TargetFormat != "edifact" {
		t.Errorf("GetByFormats returned wrong mapping: %+v", m)
	}
}

func TestMockMappingRepository_GetByFormats_NotFound(t *testing.T) {
	repo := NewMockMappingRepository()
	_, err := repo.GetByFormats(context.Background(), "x12", "json")
	if err != ErrMappingNotFound {
		t.Errorf("expected ErrMappingNotFound, got %v", err)
	}
}

func TestMockMappingRepository_GetByFormats_AllPairs(t *testing.T) {
	repo := NewMockMappingRepository()
	pairs := []struct{ src, tgt string }{
		{"x12", "edifact"}, {"edifact", "x12"}, {"x12", "xml"}, {"edifact", "xml"},
	}
	for _, p := range pairs {
		m, err := repo.GetByFormats(context.Background(), p.src, p.tgt)
		if err != nil {
			t.Errorf("GetByFormats(%q,%q) unexpected error: %v", p.src, p.tgt, err)
			continue
		}
		if m.SourceFormat != p.src || m.TargetFormat != p.tgt {
			t.Errorf("GetByFormats(%q,%q) returned %q->%q", p.src, p.tgt, m.SourceFormat, m.TargetFormat)
		}
	}
}

// ---- Postgres mapping repository tests (sqlmock) ----------------------------

func TestPostgresMappingRepository_ListActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "name", "source_format", "target_format", "active", "description", "created_at",
	}).
		AddRow("m1", "X12 -> EDIFACT", "x12", "edifact", true, "Purchase Order: X12 to UN/EDIFACT", testTime()).
		AddRow("m2", "EDIFACT -> X12", "edifact", "x12", true, "Purchase Order: UN/EDIFACT to X12", testTime()).
		AddRow("m3", "X12 -> XML", "x12", "xml", true, "Purchase Order: X12 to Generic XML", testTime()).
		AddRow("m4", "EDIFACT -> XML", "edifact", "xml", true, "Purchase Order: EDIFACT to Generic XML", testTime())

	mock.ExpectQuery(`SELECT id, name, source_format, target_format, active`).
		WillReturnRows(rows)

	repo := NewPostgresMappingRepository(db)
	mappings, err := repo.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive error: %v", err)
	}
	if len(mappings) != 4 {
		t.Errorf("expected 4 mappings, got %d", len(mappings))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}

func TestPostgresMappingRepository_GetByFormats_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "name", "source_format", "target_format", "active", "description", "created_at",
	}).AddRow("m1", "X12 -> EDIFACT", "x12", "edifact", true, "desc", testTime())

	mock.ExpectQuery(`SELECT id, name, source_format, target_format, active`).
		WithArgs("x12", "edifact").
		WillReturnRows(rows)

	repo := NewPostgresMappingRepository(db)
	m, err := repo.GetByFormats(context.Background(), "x12", "edifact")
	if err != nil {
		t.Fatalf("GetByFormats error: %v", err)
	}
	if m.SourceFormat != "x12" || m.TargetFormat != "edifact" {
		t.Errorf("unexpected mapping: %+v", m)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}

func TestPostgresMappingRepository_GetByFormats_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, name, source_format, target_format, active`).
		WithArgs("x12", "json").
		WillReturnRows(sqlmock.NewRows(nil))

	repo := NewPostgresMappingRepository(db)
	_, err = repo.GetByFormats(context.Background(), "x12", "json")
	if err != ErrMappingNotFound {
		t.Errorf("expected ErrMappingNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}
