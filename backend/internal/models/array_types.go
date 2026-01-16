package models

import (
	"database/sql/driver"
	"time"

	"github.com/lib/pq"
)

// UUIDArray is a custom type for handling UUID[] arrays in PostgreSQL
type UUIDArray []string

// Value implements the driver.Valuer interface
func (a UUIDArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return pq.Array(a).Value()
}

// Scan implements the sql.Scanner interface
func (a *UUIDArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}
	slice := (*[]string)(a)
	return pq.Array(slice).Scan(src)
}

// IntArray is a custom type for handling INTEGER[] arrays in PostgreSQL
type IntArray []int

// Value implements the driver.Valuer interface
func (a IntArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return pq.Array(a).Value()
}

// Scan implements the sql.Scanner interface
func (a *IntArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}
	slice := (*[]int)(a)
	return pq.Array(slice).Scan(src)
}

// DateArray is a custom type for handling DATE[] arrays in PostgreSQL
type DateArray []time.Time

// Value implements the driver.Valuer interface
func (a DateArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return pq.Array(a).Value()
}

// Scan implements the sql.Scanner interface
func (a *DateArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}
	slice := (*[]time.Time)(a)
	return pq.Array(slice).Scan(src)
}
