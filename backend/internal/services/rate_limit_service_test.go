package services

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRateLimitTest(t *testing.T) (*RateLimitService, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	postgresDB := &database.PostgresDB{DB: sqlxDB}
	service := NewRateLimitService(postgresDB)

	cleanup := func() {
		db.Close()
	}

	return service, mock, cleanup
}

func TestCheckOTPRateLimit_NoRequests(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	ip := "192.168.1.1"

	// Mock phone rate limit check - no previous requests
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(0, time.Now()))

	// Mock IP rate limit check - no previous requests
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(ip, "ip", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(0, time.Now()))

	err := service.CheckOTPRateLimit(phone, ip)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckOTPRateLimit_PhoneExceeded(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	ip := "192.168.1.1"
	lastRequest := time.Now().Add(-5 * time.Minute)

	// Mock phone rate limit check - 3 requests already (exceeded)
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(3, lastRequest))

	err := service.CheckOTPRateLimit(phone, ip)
	assert.Error(t, err)

	rateLimitErr, ok := err.(*RateLimitError)
	require.True(t, ok, "Error should be RateLimitError")
	assert.Equal(t, "phone", rateLimitErr.Type)
	assert.Contains(t, rateLimitErr.Message, "Too many OTP requests for this phone number")
	assert.True(t, rateLimitErr.RetryAfter.After(time.Now()))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckOTPRateLimit_IPExceeded(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	ip := "192.168.1.1"
	lastRequest := time.Now().Add(-30 * time.Minute)

	// Mock phone rate limit check - 2 requests (OK)
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(2, lastRequest))

	// Mock IP rate limit check - 10 requests (exceeded)
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(ip, "ip", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(10, lastRequest))

	err := service.CheckOTPRateLimit(phone, ip)
	assert.Error(t, err)

	rateLimitErr, ok := err.(*RateLimitError)
	require.True(t, ok, "Error should be RateLimitError")
	assert.Equal(t, "ip", rateLimitErr.Type)
	assert.Contains(t, rateLimitErr.Message, "Too many OTP requests from this IP address")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckOTPRateLimit_BelowLimit(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	ip := "192.168.1.1"
	lastRequest := time.Now().Add(-2 * time.Minute)

	// Mock phone rate limit check - 2 requests (OK)
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(2, lastRequest))

	// Mock IP rate limit check - 5 requests (OK)
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(ip, "ip", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(5, lastRequest))

	err := service.CheckOTPRateLimit(phone, ip)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordOTPRequest_Success(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	ip := "192.168.1.1"

	// Mock phone record insertion
	mock.ExpectExec("INSERT INTO otp_rate_limits").
		WithArgs(phone, "phone").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock IP record insertion
	mock.ExpectExec("INSERT INTO otp_rate_limits").
		WithArgs(ip, "ip").
		WillReturnResult(sqlmock.NewResult(2, 1))

	err := service.RecordOTPRequest(phone, ip)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordOTPRequest_PhoneOnly(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"

	// Mock phone record insertion
	mock.ExpectExec("INSERT INTO otp_rate_limits").
		WithArgs(phone, "phone").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := service.RecordOTPRequest(phone, "")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordOTPRequest_IPOnly(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ip := "192.168.1.1"

	// Mock IP record insertion
	mock.ExpectExec("INSERT INTO otp_rate_limits").
		WithArgs(ip, "ip").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := service.RecordOTPRequest("", ip)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupExpiredRateLimits_Success(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	// Mock cleanup deletion - 10 rows deleted
	mock.ExpectExec("DELETE FROM otp_rate_limits").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 10))

	rowsAffected, err := service.CleanupExpiredRateLimits()
	assert.NoError(t, err)
	assert.Equal(t, int64(10), rowsAffected)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupExpiredRateLimits_NoRows(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	// Mock cleanup deletion - 0 rows deleted
	mock.ExpectExec("DELETE FROM otp_rate_limits").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rowsAffected, err := service.CleanupExpiredRateLimits()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rowsAffected)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRateLimitStatus_Phone(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	lastRequest := time.Now().Add(-3 * time.Minute)

	// Mock rate limit status check
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(2, lastRequest))

	count, last, err := service.GetRateLimitStatus(phone, "phone")
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.WithinDuration(t, lastRequest, last, time.Second)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRateLimitStatus_IP(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ip := "192.168.1.1"
	lastRequest := time.Now().Add(-15 * time.Minute)

	// Mock rate limit status check
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(ip, "ip", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(5, lastRequest))

	count, last, err := service.GetRateLimitStatus(ip, "ip")
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.WithinDuration(t, lastRequest, last, time.Second)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsRateLimited_NotLimited(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	lastRequest := time.Now().Add(-2 * time.Minute)

	// Mock rate limit check - 2 requests (not limited)
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(2, lastRequest))

	isLimited, retryAfter, err := service.IsRateLimited(phone, "phone")
	assert.NoError(t, err)
	assert.False(t, isLimited)
	assert.True(t, retryAfter.IsZero())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsRateLimited_Limited(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"
	lastRequest := time.Now().Add(-5 * time.Minute)

	// Mock rate limit check - 3 requests (limited)
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "created_at"}).
			AddRow(3, lastRequest))

	isLimited, retryAfter, err := service.IsRateLimited(phone, "phone")
	assert.NoError(t, err)
	assert.True(t, isLimited)
	assert.True(t, retryAfter.After(time.Now()))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckOTPRateLimit_DatabaseError(t *testing.T) {
	service, mock, cleanup := setupRateLimitTest(t)
	defer cleanup()

	phone := "0771234567"

	// Mock database error
	mock.ExpectQuery("SELECT COUNT(.+) FROM otp_rate_limits").
		WithArgs(phone, "phone", sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)

	err := service.CheckOTPRateLimit(phone, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check phone rate limit")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.Equal(t, 3, config.MaxPhoneRequests)
	assert.Equal(t, 10*time.Minute, config.PhoneWindow)
	assert.Equal(t, 10, config.MaxIPRequests)
	assert.Equal(t, 1*time.Hour, config.IPWindow)
}
