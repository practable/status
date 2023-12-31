// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// Statistics connection statistics
//
// swagger:model Statistics
type Statistics struct {

	// messages per second (frames per second if video)
	Fps int64 `json:"fps,omitempty"`

	// date and time of the last message sent
	Last string `json:"last,omitempty"`

	// true if not messages ever sent on this connection
	Never bool `json:"never,omitempty"`

	// size in bytes of the last message sent
	Size int64 `json:"size,omitempty"`
}

// Validate validates this statistics
func (m *Statistics) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this statistics based on context it is used
func (m *Statistics) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *Statistics) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *Statistics) UnmarshalBinary(b []byte) error {
	var res Statistics
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
