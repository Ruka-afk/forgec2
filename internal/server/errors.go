package server

import "net/http"

type ErrorCode int

const (
	ErrNone                 ErrorCode = 0
	ErrBadRequest           ErrorCode = 40000
	ErrUnauthorized         ErrorCode = 40100
	ErrForbidden            ErrorCode = 40300
	ErrNotFound             ErrorCode = 40400
	ErrConflict             ErrorCode = 40900
	ErrRateLimited          ErrorCode = 42900
	ErrInternal             ErrorCode = 50000
	ErrServiceUnavailable   ErrorCode = 50300
	ErrDatabase             ErrorCode = 50010
	ErrConfigInvalid        ErrorCode = 50020
	ErrCryptoFailed         ErrorCode = 50030
	ErrAgentNotFound        ErrorCode = 40410
	ErrTaskNotFound         ErrorCode = 40420
	ErrPluginNotFound       ErrorCode = 40430
	ErrSessionExpired       ErrorCode = 40110
	ErrSessionRevoked       ErrorCode = 40120
	ErrAccountLocked        ErrorCode = 40130
	ErrCSRFMismatch         ErrorCode = 40310
	ErrPermissionDenied     ErrorCode = 40320
	ErrTOTPRequired         ErrorCode = 40140
	ErrTOTPInvalid          ErrorCode = 40150
	ErrBackupCodeExhausted  ErrorCode = 40160
	ErrPasswordComplexity   ErrorCode = 40010
	ErrPasswordHistory      ErrorCode = 40020
	ErrPayloadTooLarge      ErrorCode = 40030
	ErrInvalidExtension     ErrorCode = 40040
	ErrInvalidCallbackURL   ErrorCode = 40050
	ErrAgentOffline         ErrorCode = 40440
	ErrAgentBusy            ErrorCode = 40450
	ErrBuildJobFailed       ErrorCode = 50040
	ErrBuildJobNotFound     ErrorCode = 40460
	ErrListenerFailed       ErrorCode = 50050
	ErrListenerNotFound     ErrorCode = 40470
	ErrEncryptedFieldFailed ErrorCode = 50060
	ErrDatabaseMigration    ErrorCode = 50070
	ErrConfigReloadFailed   ErrorCode = 50080
	ErrEmergencyStopFailed  ErrorCode = 50090
	ErrProfileNotFound      ErrorCode = 40480
	ErrIntegrityViolation   ErrorCode = 50100
)

var errorCodeStatus = map[ErrorCode]int{
	ErrBadRequest:           http.StatusBadRequest,
	ErrUnauthorized:         http.StatusUnauthorized,
	ErrForbidden:            http.StatusForbidden,
	ErrNotFound:             http.StatusNotFound,
	ErrConflict:             http.StatusConflict,
	ErrRateLimited:          http.StatusTooManyRequests,
	ErrInternal:             http.StatusInternalServerError,
	ErrServiceUnavailable:   http.StatusServiceUnavailable,
	ErrDatabase:             http.StatusInternalServerError,
	ErrConfigInvalid:        http.StatusInternalServerError,
	ErrCryptoFailed:         http.StatusInternalServerError,
	ErrAgentNotFound:        http.StatusNotFound,
	ErrTaskNotFound:         http.StatusNotFound,
	ErrPluginNotFound:       http.StatusNotFound,
	ErrSessionExpired:       http.StatusUnauthorized,
	ErrSessionRevoked:       http.StatusUnauthorized,
	ErrAccountLocked:        http.StatusUnauthorized,
	ErrCSRFMismatch:         http.StatusForbidden,
	ErrPermissionDenied:     http.StatusForbidden,
	ErrTOTPRequired:         http.StatusUnauthorized,
	ErrTOTPInvalid:          http.StatusUnauthorized,
	ErrBackupCodeExhausted:  http.StatusUnauthorized,
	ErrPasswordComplexity:   http.StatusBadRequest,
	ErrPasswordHistory:      http.StatusBadRequest,
	ErrPayloadTooLarge:      http.StatusRequestEntityTooLarge,
	ErrInvalidExtension:     http.StatusBadRequest,
	ErrInvalidCallbackURL:   http.StatusBadRequest,
	ErrAgentOffline:         http.StatusNotFound,
	ErrAgentBusy:            http.StatusConflict,
	ErrBuildJobFailed:       http.StatusInternalServerError,
	ErrBuildJobNotFound:     http.StatusNotFound,
	ErrListenerFailed:       http.StatusInternalServerError,
	ErrListenerNotFound:     http.StatusNotFound,
	ErrEncryptedFieldFailed: http.StatusInternalServerError,
	ErrDatabaseMigration:    http.StatusInternalServerError,
	ErrConfigReloadFailed:   http.StatusInternalServerError,
	ErrEmergencyStopFailed:  http.StatusInternalServerError,
	ErrProfileNotFound:      http.StatusNotFound,
	ErrIntegrityViolation:   http.StatusInternalServerError,
}

func (e ErrorCode) HTTPStatus() int {
	if s, ok := errorCodeStatus[e]; ok {
		return s
	}
	return http.StatusInternalServerError
}
