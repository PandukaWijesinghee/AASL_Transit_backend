package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPhoneValidator(t *testing.T) {
	validator := NewPhoneValidator()
	assert.NotNil(t, validator)
}

func TestValidate_ValidNumbers(t *testing.T) {
	validator := NewPhoneValidator()

	validNumbers := []struct {
		input    string
		expected string
		name     string
	}{
		{"0771234567", "0771234567", "Standard format"},
		{"077 123 4567", "0771234567", "With spaces"},
		{"077-123-4567", "0771234567", "With dashes"},
		{"077.123.4567", "0771234567", "With dots"},
		{"(077) 123 4567", "0771234567", "With parentheses"},
		{"0701234567", "0701234567", "Mobitel 070"},
		{"0711234567", "0711234567", "Mobitel 071"},
		{"0721234567", "0721234567", "Hutch 072"},
		{"0751234567", "0751234567", "Airtel 075"},
		{"0761234567", "0761234567", "Dialog 076"},
		{"0781234567", "0781234567", "Hutch 078"},
		{"94771234567", "0771234567", "With country code"},
	}

	for _, tc := range validNumbers {
		t.Run(tc.name, func(t *testing.T) {
			sanitized, err := validator.Validate(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, sanitized)
		})
	}
}

func TestValidate_InvalidNumbers(t *testing.T) {
	validator := NewPhoneValidator()

	invalidNumbers := []struct {
		input       string
		expectedErr error
		name        string
	}{
		{"", ErrEmptyPhone, "Empty string"},
		{"123", ErrInvalidLength, "Too short"},
		{"07712345678", ErrInvalidLength, "Too long"},
		{"0791234567", ErrInvalidPrefix, "Invalid prefix 079"},
		{"0731234567", ErrInvalidPrefix, "Invalid prefix 073"},
		{"0741234567", ErrInvalidPrefix, "Invalid prefix 074"},
		{"077123456a", ErrInvalidFormat, "Contains letters"},
		{"077-123-456a", ErrInvalidFormat, "Contains letters with dashes"},
		{"077 123 456!", ErrInvalidFormat, "Contains special characters"},
		{"1234567890", ErrInvalidPrefix, "Valid length but invalid prefix"},
	}

	for _, tc := range invalidNumbers {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validator.Validate(tc.input)
			assert.Error(t, err)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestSanitize(t *testing.T) {
	validator := NewPhoneValidator()

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{"0771234567", "0771234567", "Already clean"},
		{"077 123 4567", "0771234567", "With spaces"},
		{"077-123-4567", "0771234567", "With dashes"},
		{"077.123.4567", "0771234567", "With dots"},
		{"(077) 123 4567", "0771234567", "With parentheses"},
		{"+94771234567", "0771234567", "With country code and plus"},
		{"94771234567", "0771234567", "With country code"},
		{"077-123-4567  ", "0771234567", "With trailing spaces"},
		{"  077-123-4567", "0771234567", "With leading spaces"},
		{"077 - 123 - 4567", "0771234567", "Multiple separators"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.Sanitize(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsValidPrefix(t *testing.T) {
	validator := NewPhoneValidator()

	validPrefixes := []string{
		"0701234567",
		"0711234567",
		"0721234567",
		"0751234567",
		"0761234567",
		"0771234567",
		"0781234567",
	}

	for _, phone := range validPrefixes {
		t.Run(phone[:3], func(t *testing.T) {
			assert.True(t, validator.IsValidPrefix(phone))
		})
	}

	invalidPrefixes := []string{
		"0691234567",
		"0731234567",
		"0741234567",
		"0791234567",
		"0801234567",
		"0111234567",
	}

	for _, phone := range invalidPrefixes {
		t.Run(phone[:3], func(t *testing.T) {
			assert.False(t, validator.IsValidPrefix(phone))
		})
	}

	// Edge cases
	assert.False(t, validator.IsValidPrefix("07"))
	assert.False(t, validator.IsValidPrefix(""))
}

func TestFormat(t *testing.T) {
	validator := NewPhoneValidator()

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{"0771234567", "077 123 4567", "Standard format"},
		{"077 123 4567", "077 123 4567", "Already formatted"},
		{"077-123-4567", "077 123 4567", "With dashes"},
		{"0701234567", "070 123 4567", "Mobitel 070"},
		{"0721234567", "072 123 4567", "Hutch 072"},
		{"94771234567", "077 123 4567", "With country code"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := validator.Format(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}

	// Test invalid input
	_, err := validator.Format("invalid")
	assert.Error(t, err)
}

func TestGetOperator(t *testing.T) {
	validator := NewPhoneValidator()

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{"0701234567", "Mobitel", "Mobitel 070"},
		{"0711234567", "Mobitel", "Mobitel 071"},
		{"0721234567", "Hutch", "Hutch 072"},
		{"0781234567", "Hutch", "Hutch 078"},
		{"0751234567", "Airtel", "Airtel 075"},
		{"0761234567", "Dialog", "Dialog 076"},
		{"0771234567", "Dialog", "Dialog 077"},
		{"077 123 4567", "Dialog", "Dialog with spaces"},
		{"94771234567", "Dialog", "Dialog with country code"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			operator, err := validator.GetOperator(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, operator)
		})
	}

	// Test invalid input
	_, err := validator.GetOperator("invalid")
	assert.Error(t, err)
}

func TestValidateMultiple(t *testing.T) {
	validator := NewPhoneValidator()

	phones := []string{
		"0771234567", // Valid
		"0701234567", // Valid
		"invalid",    // Invalid
		"123",        // Invalid
		"0721234567", // Valid
		"0791234567", // Invalid prefix
	}

	results := validator.ValidateMultiple(phones)

	assert.Len(t, results, 6)
	assert.Nil(t, results["0771234567"])
	assert.Nil(t, results["0701234567"])
	assert.Nil(t, results["0721234567"])
	assert.NotNil(t, results["invalid"])
	assert.NotNil(t, results["123"])
	assert.NotNil(t, results["0791234567"])
}

func TestIsValid(t *testing.T) {
	validator := NewPhoneValidator()

	validNumbers := []string{
		"0771234567",
		"077 123 4567",
		"077-123-4567",
		"0701234567",
		"94771234567",
	}

	for _, phone := range validNumbers {
		t.Run(phone, func(t *testing.T) {
			assert.True(t, validator.IsValid(phone))
		})
	}

	invalidNumbers := []string{
		"",
		"invalid",
		"123",
		"0791234567",
		"077123456a",
	}

	for _, phone := range invalidNumbers {
		t.Run(phone, func(t *testing.T) {
			assert.False(t, validator.IsValid(phone))
		})
	}
}

func TestMustValidate(t *testing.T) {
	validator := NewPhoneValidator()

	// Test valid phone
	result := validator.MustValidate("0771234567")
	assert.Equal(t, "0771234567", result)

	// Test invalid phone (should panic)
	assert.Panics(t, func() {
		validator.MustValidate("invalid")
	})
}

func TestCountryCodeHandling(t *testing.T) {
	validator := NewPhoneValidator()

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{"94771234567", "0771234567", "With 94 country code"},
		{"+94771234567", "0771234567", "With +94 country code"},
		{"94 77 123 4567", "0771234567", "With 94 and spaces"},
		{"94-77-123-4567", "0771234567", "With 94 and dashes"},
		{"0771234567", "0771234567", "Without country code"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sanitized, err := validator.Validate(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, sanitized)
		})
	}
}

func TestEdgeCases(t *testing.T) {
	validator := NewPhoneValidator()

	t.Run("Phone with only spaces", func(t *testing.T) {
		_, err := validator.Validate("     ")
		assert.Error(t, err)
	})

	t.Run("Phone with mixed separators", func(t *testing.T) {
		sanitized, err := validator.Validate("077-123 4567")
		require.NoError(t, err)
		assert.Equal(t, "0771234567", sanitized)
	})

	t.Run("Phone with unicode characters", func(t *testing.T) {
		_, err := validator.Validate("077१२३4567") // Contains Devanagari digits
		assert.Error(t, err)
	})

	t.Run("Very long input", func(t *testing.T) {
		_, err := validator.Validate("077123456789012345678901234567890")
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidLength, err)
	})
}

func TestConcurrentValidation(t *testing.T) {
	validator := NewPhoneValidator()

	done := make(chan bool)
	errors := make(chan error, 100)

	phones := []string{
		"0771234567",
		"0701234567",
		"0721234567",
		"0751234567",
		"0761234567",
	}

	// Validate 100 phones concurrently
	for i := 0; i < 100; i++ {
		go func(phone string) {
			_, err := validator.Validate(phone)
			if err != nil {
				errors <- err
			}
			done <- true
		}(phones[i%len(phones)])
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	close(errors)
	assert.Empty(t, errors)
}

func BenchmarkValidate(b *testing.B) {
	validator := NewPhoneValidator()
	phone := "0771234567"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Validate(phone)
	}
}

func BenchmarkSanitize(b *testing.B) {
	validator := NewPhoneValidator()
	phone := "077-123-4567"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Sanitize(phone)
	}
}

func BenchmarkFormat(b *testing.B) {
	validator := NewPhoneValidator()
	phone := "0771234567"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Format(phone)
	}
}
