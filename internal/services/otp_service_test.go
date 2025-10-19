package services

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOTPService(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)

	assert.NotNil(t, service)
}

func TestGenerateOTP(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"

	// Expect invalidate query
	mock.ExpectExec("UPDATE otp_verifications").
		WithArgs(phone).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect insert query
	mock.ExpectExec("INSERT INTO otp_verifications").
		WithArgs(phone, sqlmock.AnyArg(), sqlmock.AnyArg(), MaxOTPAttempts).
		WillReturnResult(sqlmock.NewResult(1, 1))

	otp, err := service.GenerateOTP(phone)
	require.NoError(t, err)
	assert.Len(t, otp, 6)
	assert.Regexp(t, "^[0-9]{6}$", otp)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateOTP_Uniqueness(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"

	otps := make(map[string]bool)

	for i := 0; i < 100; i++ {
		// Expect invalidate query
		mock.ExpectExec("UPDATE otp_verifications").
			WithArgs(phone).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Expect insert query
		mock.ExpectExec("INSERT INTO otp_verifications").
			WithArgs(phone, sqlmock.AnyArg(), sqlmock.AnyArg(), MaxOTPAttempts).
			WillReturnResult(sqlmock.NewResult(1, 1))

		otp, err := service.GenerateOTP(phone)
		require.NoError(t, err)
		otps[otp] = true
	}

	// Should generate different OTPs (at least 80% unique)
	assert.Greater(t, len(otps), 80)
}

func TestValidateOTP_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"
	otp := "123456"
	expiresAt := time.Now().Add(5 * time.Minute)

	// Mock get OTP record
	rows := sqlmock.NewRows([]string{"id", "phone", "otp_code", "purpose", "created_at", "expires_at", "verified", "verified_at", "attempts", "max_attempts", "ip_address", "user_agent"}).
		AddRow(1, phone, otp, "authentication", time.Now(), expiresAt, false, nil, 0, 3, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
		WithArgs(phone).
		WillReturnRows(rows)

	// Mock increment attempts
	mock.ExpectExec("UPDATE otp_verifications SET attempts").
		WithArgs(phone).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock mark as verified
	mock.ExpectExec("UPDATE otp_verifications SET verified").
		WithArgs(sqlmock.AnyArg(), phone).
		WillReturnResult(sqlmock.NewResult(0, 1))

	valid, err := service.ValidateOTP(phone, otp)
	require.NoError(t, err)
	assert.True(t, valid)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateOTP_InvalidCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"
	correctOTP := "123456"
	wrongOTP := "654321"
	expiresAt := time.Now().Add(5 * time.Minute)

	// Mock get OTP record
	rows := sqlmock.NewRows([]string{"id", "phone", "otp_code", "purpose", "created_at", "expires_at", "verified", "verified_at", "attempts", "max_attempts", "ip_address", "user_agent"}).
		AddRow(1, phone, correctOTP, "authentication", time.Now(), expiresAt, false, nil, 0, 3, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
		WithArgs(phone).
		WillReturnRows(rows)

	// Mock increment attempts
	mock.ExpectExec("UPDATE otp_verifications SET attempts").
		WithArgs(phone).
		WillReturnResult(sqlmock.NewResult(0, 1))

	valid, err := service.ValidateOTP(phone, wrongOTP)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Equal(t, ErrOTPInvalid, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateOTP_Expired(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"
	otp := "123456"
	expiresAt := time.Now().Add(-1 * time.Minute) // Expired 1 minute ago

	// Mock get OTP record
	rows := sqlmock.NewRows([]string{"id", "phone", "otp_code", "purpose", "created_at", "expires_at", "verified", "verified_at", "attempts", "max_attempts", "ip_address", "user_agent"}).
		AddRow(1, phone, otp, "authentication", time.Now(), expiresAt, false, nil, 0, 3, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
		WithArgs(phone).
		WillReturnRows(rows)

	valid, err := service.ValidateOTP(phone, otp)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Equal(t, ErrOTPExpired, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateOTP_MaxAttemptsExceeded(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"
	otp := "123456"
	expiresAt := time.Now().Add(5 * time.Minute)

	// Mock get OTP record with max attempts already reached
	rows := sqlmock.NewRows([]string{"id", "phone", "otp_code", "purpose", "created_at", "expires_at", "verified", "verified_at", "attempts", "max_attempts", "ip_address", "user_agent"}).
		AddRow(1, phone, otp, "authentication", time.Now(), expiresAt, false, nil, 3, 3, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
		WithArgs(phone).
		WillReturnRows(rows)

	valid, err := service.ValidateOTP(phone, otp)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Equal(t, ErrMaxAttemptsExceeded, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateOTP_AlreadyUsed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"
	otp := "123456"
	expiresAt := time.Now().Add(5 * time.Minute)

	// Mock get OTP record that's already verified
	rows := sqlmock.NewRows([]string{"id", "phone", "otp_code", "purpose", "created_at", "expires_at", "verified", "verified_at", "attempts", "max_attempts", "ip_address", "user_agent"}).
		AddRow(1, phone, otp, "authentication", time.Now(), expiresAt, true, time.Now(), 1, 3, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
		WithArgs(phone).
		WillReturnRows(rows)

	valid, err := service.ValidateOTP(phone, otp)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Equal(t, ErrOTPAlreadyUsed, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateOTP_NoOTPFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"
	otp := "123456"

	// Mock get OTP record returning no rows
	mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
		WithArgs(phone).
		WillReturnError(sql.ErrNoRows)

	valid, err := service.ValidateOTP(phone, otp)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Equal(t, ErrNoOTPFound, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRemainingAttempts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"
	expiresAt := time.Now().Add(5 * time.Minute)

	tests := []struct {
		name           string
		attempts       int
		expectedRemain int
	}{
		{"No attempts yet", 0, 3},
		{"One attempt", 1, 2},
		{"Two attempts", 2, 1},
		{"Max attempts", 3, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := sqlmock.NewRows([]string{"id", "phone", "otp_code", "purpose", "created_at", "expires_at", "verified", "verified_at", "attempts", "max_attempts", "ip_address", "user_agent"}).
				AddRow(1, phone, "123456", "authentication", time.Now(), expiresAt, false, nil, tc.attempts, 3, nil, nil)

			mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
				WithArgs(phone).
				WillReturnRows(rows)

			remaining, err := service.GetRemainingAttempts(phone)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedRemain, remaining)
		})
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsOTPExpired(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)
	phone := "0771234567"

	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{"Not expired", time.Now().Add(5 * time.Minute), false},
		{"Expired", time.Now().Add(-1 * time.Minute), true},
		{"Just expired", time.Now().Add(-1 * time.Millisecond), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := sqlmock.NewRows([]string{"id", "phone", "otp_code", "purpose", "created_at", "expires_at", "verified", "verified_at", "attempts", "max_attempts", "ip_address", "user_agent"}).
				AddRow(1, phone, "123456", "authentication", time.Now(), tc.expiresAt, false, nil, 0, 3, nil, nil)

			mock.ExpectQuery("SELECT (.+) FROM otp_verifications").
				WithArgs(phone).
				WillReturnRows(rows)

			expired, err := service.IsOTPExpired(phone)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, expired)
		})
	}
}

func TestCleanupExpiredOTPs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)

	mock.ExpectExec("DELETE FROM otp_verifications").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 5))

	rowsAffected, err := service.CleanupExpiredOTPs()
	require.NoError(t, err)
	assert.Equal(t, int64(5), rowsAffected)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupOldOTPs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	service := NewOTPService(mockDB)

	mock.ExpectExec("DELETE FROM otp_verifications").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 10))

	rowsAffected, err := service.CleanupOldOTPs(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(10), rowsAffected)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateRandomOTP(t *testing.T) {
	for i := 0; i < 100; i++ {
		otp, err := generateRandomOTP()
		require.NoError(t, err)
		assert.Len(t, otp, 6)
		assert.Regexp(t, "^[0-9]{6}$", otp)
	}
}

// mockDatabase implements the database.DB interface for testing
type mockDatabase struct {
	db *sql.DB
}

func (m *mockDatabase) Get(dest interface{}, query string, args ...interface{}) error {
	return fmt.Errorf("Get not implemented in mock")
}

func (m *mockDatabase) Select(dest interface{}, query string, args ...interface{}) error {
	return fmt.Errorf("Select not implemented in mock")
}

func (m *mockDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}

func (m *mockDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

func (m *mockDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return m.db.QueryRow(query, args...)
}

func (m *mockDatabase) Close() error {
	return m.db.Close()
}

func (m *mockDatabase) Ping() error {
	return m.db.Ping()
}
