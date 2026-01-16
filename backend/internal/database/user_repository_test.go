package database

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "", "", "", "",
				"", "", []byte(`{"passenger"}`), "active", false,
				true, false, now, now,
			))

		user, err := repo.CreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "active", user.Status)
		assert.True(t, user.PhoneVerified)
		assert.False(t, user.ProfileCompleted)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg()).
			WillReturnError(fmt.Errorf("database error"))

		user, err := repo.CreateUser(phone)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to create user")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Duplicate Phone", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg()).
			WillReturnError(fmt.Errorf("duplicate key value violates unique constraint"))

		user, err := repo.CreateUser(phone)
		assert.Error(t, err)
		assert.Nil(t, user)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetUserByPhone(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "John", "Doe", "john@example.com", "123 Main St",
				"Colombo", "10100", []byte(`{"passenger"}`), "active", true,
				true, true, now, now,
			))

		user, err := repo.GetUserByPhone(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "John", user.FirstName)
		assert.Equal(t, "Doe", user.LastName)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByPhone(phone)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(fmt.Errorf("database error"))

		user, err := repo.GetUserByPhone(phone)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to fetch user")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetUserByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		phone := "+94712345678"
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "Jane", "Smith", "jane@example.com", "456 Oak Ave",
				"Kandy", "20000", []byte(`{"passenger","driver"}`), "active", true,
				true, false, now, now,
			))

		user, err := repo.GetUserByID(userID)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "Jane", user.FirstName)
		assert.Equal(t, "Smith", user.LastName)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByID(userID)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateProfile(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		firstName := "John"
		lastName := "Doe"
		email := "john.doe@example.com"
		address := "123 Main Street"
		city := "Colombo"
		postalCode := "10100"

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs(firstName, lastName, email, address, city, postalCode, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfile(userID, firstName, lastName, email, address, city, postalCode)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs("John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.UpdateProfile(userID, "John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs("John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100", sqlmock.AnyArg(), userID).
			WillReturnError(fmt.Errorf("database error"))

		err := repo.UpdateProfile(userID, "John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update profile")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestIsProfileComplete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Complete Profile", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "john@example.com", "123 Main St"))

		isComplete, err := repo.IsProfileComplete(userID)
		require.NoError(t, err)
		assert.True(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Incomplete Profile - Missing Name", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("", "Doe", "john@example.com", "123 Main St"))

		isComplete, err := repo.IsProfileComplete(userID)
		require.NoError(t, err)
		assert.False(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Incomplete Profile - Missing Email", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "", "123 Main St"))

		isComplete, err := repo.IsProfileComplete(userID)
		require.NoError(t, err)
		assert.False(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		isComplete, err := repo.IsProfileComplete(userID)
		assert.Error(t, err)
		assert.False(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateProfileCompletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Mark Complete", func(t *testing.T) {
		userID := uuid.New()

		// First query to check profile fields
		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "john@example.com", "123 Main St"))

		// Update query to set profile_completed = true
		mock.ExpectExec(`UPDATE users SET profile_completed`).
			WithArgs(true, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfileCompletion(userID)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Mark Incomplete - Missing Fields", func(t *testing.T) {
		userID := uuid.New()

		// Query returns incomplete profile (missing email)
		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "", "123 Main St"))

		// Update query to set profile_completed = false
		mock.ExpectExec(`UPDATE users SET profile_completed`).
			WithArgs(false, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfileCompletion(userID)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		err := repo.UpdateProfileCompletion(userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetOrCreateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Get Existing User", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "John", "Doe", "john@example.com", "123 Main St",
				"Colombo", "10100", []byte(`{"passenger"}`), "active", true,
				true, true, now, now,
			))

		user, isNew, err := repo.GetOrCreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.False(t, isNew)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "John", user.FirstName)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Create New User", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		// First query returns no rows (user doesn't exist)
		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(sql.ErrNoRows)

		// Then insert new user
		mock.ExpectQuery(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "", "", "", "",
				"", "", []byte(`{"passenger"}`), "active", false,
				true, false, now, now,
			))

		user, isNew, err := repo.GetOrCreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.True(t, isNew)
		assert.Equal(t, phone, user.Phone)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateUserStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Update to Active", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("active", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateUserStatus(userID, "active")
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Update to Suspended", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("suspended", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateUserStatus(userID, "suspended")
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("banned", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.UpdateUserStatus(userID, "banned")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestAddUserRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		role := "driver"

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs(role, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.AddUserRole(userID, role)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs("admin", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.AddUserRole(userID, "admin")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestRemoveUserRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		role := "driver"

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs(role, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.RemoveUserRole(userID, role)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs("passenger", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.RemoveUserRole(userID, "passenger")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestListUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		now := time.Now()
		user1ID := uuid.New()
		user2ID := uuid.New()

		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).
				AddRow(user1ID, "+94712345678", "John", "Doe", "john@example.com", "123 Main St",
					"Colombo", "10100", []byte(`{"passenger"}`), "active", true, true, true, now, now).
				AddRow(user2ID, "+94723456789", "Jane", "Smith", "jane@example.com", "456 Oak Ave",
					"Kandy", "20000", []byte(`{"passenger","driver"}`), "active", true, true, false, now, now))

		users, err := repo.ListUsers(10, 0)
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "John", users[0].FirstName)
		assert.Equal(t, "Jane", users[1].FirstName)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Empty Result", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}))

		users, err := repo.ListUsers(10, 0)
		require.NoError(t, err)
		assert.Len(t, users, 0)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnError(fmt.Errorf("database error"))

		users, err := repo.ListUsers(10, 0)
		assert.Error(t, err)
		assert.Nil(t, users)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestCountUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: db}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

		count, err := repo.CountUsers()
		require.NoError(t, err)
		assert.Equal(t, int64(42), count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Zero Users", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		count, err := repo.CountUsers()
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnError(fmt.Errorf("database error"))

		count, err := repo.CountUsers()
		assert.Error(t, err)
		assert.Equal(t, int64(0), count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

// Mock database implementation for testing
type mockDatabase struct {
	db *sql.DB
}

func (m *mockDatabase) Get(dest interface{}, query string, args ...interface{}) error {
	return fmt.Errorf("Get not implemented in mock")
}

func (m *mockDatabase) Select(dest interface{}, query string, args ...interface{}) error {
	return fmt.Errorf("Select not implemented in mock")
}

func (m *mockDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

func (m *mockDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return m.db.QueryRow(query, args...)
}

func (m *mockDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}

func (m *mockDatabase) Close() error {
	return m.db.Close()
}

func (m *mockDatabase) Ping() error {
	return m.db.Ping()
}
