package main

// boolOrStringValue is a custom flag type that can be a boolean or a non-empty string.
// We need it to support nfd-worker --export, which can be set (true) or accept
// an export value (--export=file.json). It satisfies the flag.Value interface.
type boolOrStringValue struct {
	// Fistinguish "not set" (false) from "set as a boolean" (true).
	IsSet bool

	// Value holds the string provided.
	// If the flag is used as a boolean (e.g., -flag), this value will be "true".
	Value string
}

// String is the method to format the flag's value for printing.
// It's part of the flag.Value interface.
func (b *boolOrStringValue) String() string {
	if !b.IsSet {
		// Represents the "not set" state
		return ""
	}
	return b.Value
}

// Set parses the flag's value from the command line.
func (b *boolOrStringValue) Set(s string) error {
	// When Set is called, we know the flag was present on the command line.
	b.IsSet = true
	b.Value = s
	return nil
}

// IsBoolFlag is an optional method that makes the flag behave like a boolean.
func (b *boolOrStringValue) IsBoolFlag() bool {
	return true
}
