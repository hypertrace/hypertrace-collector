package json

import "encoding/json"

// MarshalUnmarshaler marshals and unmarshals from/to string
type MarshalUnmarshaler interface {
	UnmarshalFromString(str string, v interface{}) error
	MarshalToString(v interface{}) (string, error)
}

// DefaultMarshalUnmarshaler is the default implementation for MarshalUnmarshaler
// using the standard library
var DefaultMarshalUnmarshaler MarshalUnmarshaler = &defaultImpl{}

type defaultImpl struct{}

func (u *defaultImpl) UnmarshalFromString(str string, v interface{}) error {
	return json.Unmarshal([]byte(str), v)
}
func (u *defaultImpl) MarshalToString(v interface{}) (string, error) {
	mv, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(mv), nil
}
