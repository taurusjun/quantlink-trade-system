package indicators

import "errors"

var (
	// ErrIndicatorTypeNotFound is returned when an indicator type is not registered
	ErrIndicatorTypeNotFound = errors.New("indicator type not found")

	// ErrInvalidConfig is returned when indicator configuration is invalid
	ErrInvalidConfig = errors.New("invalid indicator configuration")

	// ErrInsufficientData is returned when there's not enough data
	ErrInsufficientData = errors.New("insufficient data")

	// ErrInvalidParameter is returned when a parameter value is invalid
	ErrInvalidParameter = errors.New("invalid parameter")
)
