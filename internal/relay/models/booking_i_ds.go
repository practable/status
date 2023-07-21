// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// BookingIDs Set of booking IDs (bids)
//
// swagger:model BookingIDs
type BookingIDs struct {

	// list bids in string format
	// Required: true
	BookingIds []string `json:"booking_ids"`
}

// Validate validates this booking i ds
func (m *BookingIDs) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateBookingIds(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *BookingIDs) validateBookingIds(formats strfmt.Registry) error {

	if err := validate.Required("booking_ids", "body", m.BookingIds); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this booking i ds based on context it is used
func (m *BookingIDs) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *BookingIDs) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *BookingIDs) UnmarshalBinary(b []byte) error {
	var res BookingIDs
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
