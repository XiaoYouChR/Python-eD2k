package goed2k

type JED2KError struct {
	Cause error
	EC    BaseErrorCode
}

func NewError(ec BaseErrorCode) *JED2KError {
	return &JED2KError{EC: ec}
}

func WrapError(cause error, ec BaseErrorCode) *JED2KError {
	return &JED2KError{Cause: cause, EC: ec}
}

func (e *JED2KError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return e.EC.Description()
	}
	return e.Cause.Error() + " " + e.EC.Description()
}

func (e *JED2KError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *JED2KError) ErrorCode() BaseErrorCode {
	if e == nil {
		return nil
	}
	return e.EC
}
