package errs

// InvalidCredentialsError ... An error emitted when credentials are invalid.
type InvalidCredentialsError struct {
}

func (ice InvalidCredentialsError) Error() string {
	return "the credentials provided were invalid"
}

// IsInvalidCredentialsError ... Checks if an error is a InvalidCredentialsError.
func IsInvalidCredentialsError(err error) bool {
	_, ok := err.(*InvalidCredentialsError)

	return ok
}

type FxError uint32

const (
	// ErrSSHQuotaExceeded ...
	// Extends the default SFTP server to return a quota exceeded error to the client.
	//
	// @see https://tools.ietf.org/id/draft-ietf-secsh-filexfer-13.txt
	ErrSSHQuotaExceeded = FxError(15)
)

func (e FxError) Error() string {
	switch e {
	case ErrSSHQuotaExceeded:
		return "Quota Exceeded"
	default:
		return "Failure"
	}
}
