// Copyright (c) 2025 Visvasity LLC

package tests

import (
	"math/rand"
	"reflect"
)

// Helper function to generate random strings
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func randomize(input interface{}) {
	// Get the reflection Value of the input
	v := reflect.ValueOf(input)

	// Handle nil or invalid input
	if !v.IsValid() || v.Kind() == reflect.Ptr && v.IsNil() {
		return
	}

	// Dereference pointer if necessary
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Check if the value can be modified
	if !v.CanSet() {
		return
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(rand.Int63n(1000)) // Random int between 0 and 999

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(rand.Intn(1000))) // Random uint between 0 and 999

	case reflect.Float32, reflect.Float64:
		v.SetFloat(rand.Float64() * 1000) // Random float between 0 and 1000

	case reflect.Bool:
		v.SetBool(rand.Intn(2) == 1) // Random true/false

	case reflect.String:
		v.SetString(randomString(10)) // Random string of length 10

	case reflect.Slice:
		if v.Len() > 0 {
			for i := 0; i < v.Len(); i++ {
				randomize(v.Index(i).Addr().Interface())
			}
		}

	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			randomize(v.Index(i).Addr().Interface())
		}

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				randomize(v.Field(i).Addr().Interface())
			}
		}
	}
}

func deepEqual(a, b interface{}) bool {
	// If both are nil, they're equal
	if a == nil && b == nil {
		return true
	}
	// If one is nil and the other isn't, they're not equal
	if a == nil || b == nil {
		return false
	}

	// Get reflection values
	valA := reflect.ValueOf(a)
	valB := reflect.ValueOf(b)

	// Handle pointers by dereferencing them
	if valA.Kind() == reflect.Ptr {
		if valA.IsNil() && valB.IsNil() {
			return true
		}
		if valA.IsNil() || valB.IsNil() {
			return false
		}
		valA = valA.Elem()
	}
	if valB.Kind() == reflect.Ptr {
		valB = valB.Elem()
	}

	// Types must match
	if valA.Type() != valB.Type() {
		return false
	}

	// Handle different kinds
	switch valA.Kind() {
	case reflect.Struct:
		// Compare each field
		for i := 0; i < valA.NumField(); i++ {
			fieldA := valA.Field(i)
			fieldB := valB.Field(i)

			if !deepEqual(fieldA.Interface(), fieldB.Interface()) {
				return false
			}
		}
		return true

	case reflect.Slice, reflect.Array:
		// Lengths must match
		if valA.Len() != valB.Len() {
			return false
		}
		// Compare each element
		for i := 0; i < valA.Len(); i++ {
			if !deepEqual(valA.Index(i).Interface(), valB.Index(i).Interface()) {
				return false
			}
		}
		return true

	case reflect.Map:
		if valA.Len() != valB.Len() {
			return false
		}
		// Check each key-value pair
		for _, key := range valA.MapKeys() {
			valBValue := valB.MapIndex(key)
			if !valBValue.IsValid() { // Key doesn't exist in B
				return false
			}
			if !deepEqual(valA.MapIndex(key).Interface(), valBValue.Interface()) {
				return false
			}
		}
		return true

	case reflect.Interface:
		return deepEqual(valA.Elem().Interface(), valB.Elem().Interface())

	default:
		// For basic types, use reflect.DeepEqual
		return reflect.DeepEqual(a, b)
	}
}
