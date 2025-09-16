package validator

import (
	"regexp"
)

type Validator interface {
	Valid() bool
	AddError(string, string)
	Check(bool, string, string)
	CheckEmail(string, string, string)
}

// Declare a regular expression for sanity checking the format of email addresses
// pattern is taken from https://html.spec.whatwg.org/#valid-e-mail-address.
var (
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

// Define a new Validator type which contains a map of validation errors.
type validator struct {
	Errors map[string]string
}

func New() *validator {
	return &validator{Errors: make(map[string]string)}
}

func (v *validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

func (v *validator) CheckEmail(email, key, message string) {
	if !EmailRX.MatchString(email) {
		v.AddError(key, message)
	}
}

// Generic function which returns true if a specific value is in a list.
func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	for i := range permittedValues {
		if value == permittedValues[i] {
			return true
		}
	}
	return false
}

// Generic function which returns true if all values in a slice are unique.
func Unique[T comparable](values []T) bool {
	uniqueValues := make(map[T]bool)
	for _, value := range values {
		uniqueValues[value] = true
	}
	return len(values) == len(uniqueValues)
}
