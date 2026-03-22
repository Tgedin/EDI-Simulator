package storage

// repositoryError is a simple error type used by storage implementations.
type repositoryError struct {
	msg string
}

func (e *repositoryError) Error() string {
	return e.msg
}

// Common storage errors
var (
	ErrInvalidMessage      error = &repositoryError{"invalid message"}
	ErrMessageNotFound     error = &repositoryError{"message not found"}
	ErrInvalidTransaction  error = &repositoryError{"invalid transaction"}
	ErrMappingNotFound     error = &repositoryError{"mapping not found"}
	ErrPartnerNotFound     error = &repositoryError{"trading partner not found"}
)
