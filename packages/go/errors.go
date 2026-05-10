package agentvenv

// Error is the single concrete error type this package returns. Compare
// against the package-level sentinel vars (ErrEnvironmentNotFound, ...) using
// errors.Is. A non-nil Cause is reported by errors.Unwrap.
type Error struct {
	Kind  string
	Msg   string
	Cause error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil agentvenv.Error>"
	}
	if e.Msg == "" {
		return e.Kind
	}
	return e.Kind + ": " + e.Msg
}

func (e *Error) Unwrap() error { return e.Cause }

// Is reports a match when target is an *Error with the same Kind. This
// makes the package-level sentinels (which carry only a Kind) effective
// targets for errors.Is regardless of the wrapped Msg or Cause.
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	if !ok {
		return false
	}
	return other.Kind == e.Kind
}

// Sentinel errors. Compare with errors.Is. The Kind strings come from
// spec/errors.md.
var (
	ErrEnvironmentNotFound        = &Error{Kind: "EnvironmentNotFound"}
	ErrEnvironmentAlreadyExists   = &Error{Kind: "EnvironmentAlreadyExists"}
	ErrAdapterMismatch            = &Error{Kind: "AdapterMismatch"}
	ErrProfileSetupFailed         = &Error{Kind: "ProfileSetupFailed"}
	ErrRegistryUnavailable        = &Error{Kind: "RegistryUnavailable"}
	ErrCredentialsMissing         = &Error{Kind: "CredentialsMissing"}
	ErrAdapterUnavailable         = &Error{Kind: "AdapterUnavailable"}
	ErrCleanupFailed              = &Error{Kind: "CleanupFailed"}
	ErrInvalidEnvironmentSpec     = &Error{Kind: "InvalidEnvironmentSpec"}
	ErrInternalInvariantViolation = &Error{Kind: "InternalInvariantViolation"}
)

// newErr returns an *Error wrapping the sentinel's Kind together with a
// message and optional cause.
func newErr(sentinel *Error, msg string, cause error) *Error {
	return &Error{Kind: sentinel.Kind, Msg: msg, Cause: cause}
}
