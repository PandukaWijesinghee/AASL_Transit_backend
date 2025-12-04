package validator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// ErrInvalidLength indicates phone number length is not 10 digits
	ErrInvalidLength = errors.New("phone number must be exactly 10 digits")

	// ErrInvalidPrefix indicates phone number doesn't start with valid Sri Lankan prefix
	ErrInvalidPrefix = errors.New("phone number must start with 070, 071, 072, 074, 075, 076, 077, 078, or 079")

	// ErrInvalidFormat indicates phone number contains invalid characters
	ErrInvalidFormat = errors.New("phone number can only contain digits")

	// ErrEmptyPhone indicates phone number is empty
	ErrEmptyPhone = errors.New("phone number cannot be empty")
)

// validPrefixes contains all valid Sri Lankan mobile operator prefixes
var validPrefixes = []string{
	"070", // Mobitel
	"071", // Mobitel
	"072", // Hutch
	"074", // Dialog
	"075", // Airtel
	"076", // Dialog
	"077", // Dialog
	"078", // Hutch
	"079", // Dialog
	"067", // test
}

// phoneRegex matches digits only
var phoneRegex = regexp.MustCompile(`^\d+$`)

// PhoneValidator handles phone number validation
type PhoneValidator struct{}

// NewPhoneValidator creates a new phone validator instance
func NewPhoneValidator() *PhoneValidator {
	return &PhoneValidator{}
}

// Validate validates a Sri Lankan phone number
// Accepts format: 0771234567 or 077 123 4567 or 077-123-4567
// Returns sanitized phone number (digits only) and error if invalid
func (v *PhoneValidator) Validate(phone string) (string, error) {
	// Check if empty
	if phone == "" {
		return "", ErrEmptyPhone
	}

	// Sanitize input
	sanitized := v.Sanitize(phone)

	// Check if contains only digits
	if !phoneRegex.MatchString(sanitized) {
		return "", ErrInvalidFormat
	}

	// Check length
	if len(sanitized) != 10 {
		return "", ErrInvalidLength
	}

	// Check prefix
	if !v.IsValidPrefix(sanitized) {
		return "", ErrInvalidPrefix
	}

	return sanitized, nil
}

// Sanitize removes all non-digit characters from phone number
func (v *PhoneValidator) Sanitize(phone string) string {
	// Remove spaces, dashes, parentheses, and other common separators
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, ".", "")

	// Remove country code if present (94)
	if strings.HasPrefix(phone, "94") && len(phone) == 11 {
		phone = "0" + phone[2:] // Replace 94 with 0
	}

	return phone
}

// IsValidPrefix checks if phone number has a valid Sri Lankan mobile prefix
func (v *PhoneValidator) IsValidPrefix(phone string) bool {
	if len(phone) < 3 {
		return false
	}

	prefix := phone[:3]
	for _, validPrefix := range validPrefixes {
		if prefix == validPrefix {
			return true
		}
	}

	return false
}

// Format formats a phone number in the standard display format: 07X XXX XXXX
func (v *PhoneValidator) Format(phone string) (string, error) {
	// Validate first
	sanitized, err := v.Validate(phone)
	if err != nil {
		return "", err
	}

	// Format as: 07X XXX XXXX
	return fmt.Sprintf("%s %s %s",
		sanitized[0:3],  // 07X
		sanitized[3:6],  // XXX
		sanitized[6:10], // XXXX
	), nil
}

// GetOperator returns the mobile operator name based on prefix
func (v *PhoneValidator) GetOperator(phone string) (string, error) {
	sanitized, err := v.Validate(phone)
	if err != nil {
		return "", err
	}

	prefix := sanitized[:3]
	switch prefix {
	case "070", "071":
		return "Mobitel", nil
	case "072", "078":
		return "Hutch", nil
	case "075":
		return "Airtel", nil
	case "076", "077":
		return "Dialog", nil
	default:
		return "", ErrInvalidPrefix
	}
}

// ValidateMultiple validates multiple phone numbers at once
// Returns a map of phone number to error (nil if valid)
func (v *PhoneValidator) ValidateMultiple(phones []string) map[string]error {
	results := make(map[string]error, len(phones))
	for _, phone := range phones {
		_, err := v.Validate(phone)
		results[phone] = err
	}
	return results
}

// IsValid is a convenience method that returns true if phone is valid
func (v *PhoneValidator) IsValid(phone string) bool {
	_, err := v.Validate(phone)
	return err == nil
}

// MustValidate validates and panics if invalid (use for testing only)
func (v *PhoneValidator) MustValidate(phone string) string {
	sanitized, err := v.Validate(phone)
	if err != nil {
		panic(fmt.Sprintf("invalid phone number %s: %v", phone, err))
	}
	return sanitized
}
