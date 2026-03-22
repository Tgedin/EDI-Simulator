package storage

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/theo-gedin/edi-simulator/internal/models"
)

func TestPostgresMessageRepositoryUpdateStatusNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresMessageRepository(db)

	// Expect execution and return no rows affected
	mock.ExpectExec("UPDATE messages").
		WithArgs(models.StatusSent, "non-existent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.UpdateStatus(context.Background(), "non-existent", models.StatusSent)
	if err != ErrMessageNotFound {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPostgresMessageRepositoryUpdateStatusSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresMessageRepository(db)

	mock.ExpectExec("UPDATE messages").
		WithArgs(models.StatusReceived, "msg-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.UpdateStatus(context.Background(), "msg-123", models.StatusReceived)
	if err != nil {
		t.Errorf("expected nil error on success, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPostgresMessageRepositoryUpdateMetadataSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresMessageRepository(db)
	xmlSnapshot := "<Document/>"
	jsonVal := `"` + xmlSnapshot + `"`

	mock.ExpectExec("UPDATE messages").
		WithArgs(jsonVal, "msg-abc").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.UpdateMetadata(context.Background(), "msg-abc", xmlSnapshot)
	if err != nil {
		t.Errorf("expected nil error on success, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPostgresMessageRepositoryUpdateMetadataNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresMessageRepository(db)
	xmlSnapshot := "<Document/>"
	jsonVal := `"` + xmlSnapshot + `"`

	mock.ExpectExec("UPDATE messages").
		WithArgs(jsonVal, "nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.UpdateMetadata(context.Background(), "nonexistent", xmlSnapshot)
	if err != ErrMessageNotFound {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
