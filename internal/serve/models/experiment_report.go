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

// ExperimentReport Report on the Status of an experiment
//
// swagger:model ExperimentReport
type ExperimentReport struct {

	// is it set as available on the booking system?
	// Required: true
	Available *bool `json:"available"`

	// first checked
	// Required: true
	FirstChecked *string `json:"first_checked"`

	// number of health events recorded
	HealthEvents int64 `json:"health_events,omitempty"`

	// healthy
	// Required: true
	Healthy *bool `json:"healthy"`

	// jump ok
	// Required: true
	JumpOk *bool `json:"jump_ok"`

	// jump report
	JumpReport *JumpReport `json:"jump_report,omitempty"`

	// last checked jump
	// Required: true
	LastCheckedJump *string `json:"last_checked_jump"`

	// last checked streams
	// Required: true
	LastCheckedStreams *string `json:"last_checked_streams"`

	// last found in manifest
	// Required: true
	LastFoundInManifest *string `json:"last_found_in_manifest"`

	// name of the resouce in the manifest
	// Example: r-spin30
	// Required: true
	ResourceName *string `json:"resource_name"`

	// stream ok
	// Required: true
	StreamOk map[string]bool `json:"stream_ok"`

	// stream reports
	// Required: true
	StreamReports map[string]StreamReport `json:"stream_reports"`

	// defaults to true for required stream, false currently undefined, but kept as a map for consistenct with stream_reports and stream_ok
	// Required: true
	StreamRequired map[string]bool `json:"stream_required"`

	// topic stub in stream names
	// Example: spin30
	// Required: true
	TopicName *string `json:"topic_name"`
}

// Validate validates this experiment report
func (m *ExperimentReport) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateAvailable(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateFirstChecked(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateHealthy(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateJumpOk(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateJumpReport(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateLastCheckedJump(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateLastCheckedStreams(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateLastFoundInManifest(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateResourceName(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateStreamOk(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateStreamReports(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateStreamRequired(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateTopicName(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ExperimentReport) validateAvailable(formats strfmt.Registry) error {

	if err := validate.Required("available", "body", m.Available); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateFirstChecked(formats strfmt.Registry) error {

	if err := validate.Required("first_checked", "body", m.FirstChecked); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateHealthy(formats strfmt.Registry) error {

	if err := validate.Required("healthy", "body", m.Healthy); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateJumpOk(formats strfmt.Registry) error {

	if err := validate.Required("jump_ok", "body", m.JumpOk); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateJumpReport(formats strfmt.Registry) error {
	if swag.IsZero(m.JumpReport) { // not required
		return nil
	}

	if m.JumpReport != nil {
		if err := m.JumpReport.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("jump_report")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("jump_report")
			}
			return err
		}
	}

	return nil
}

func (m *ExperimentReport) validateLastCheckedJump(formats strfmt.Registry) error {

	if err := validate.Required("last_checked_jump", "body", m.LastCheckedJump); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateLastCheckedStreams(formats strfmt.Registry) error {

	if err := validate.Required("last_checked_streams", "body", m.LastCheckedStreams); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateLastFoundInManifest(formats strfmt.Registry) error {

	if err := validate.Required("last_found_in_manifest", "body", m.LastFoundInManifest); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateResourceName(formats strfmt.Registry) error {

	if err := validate.Required("resource_name", "body", m.ResourceName); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateStreamOk(formats strfmt.Registry) error {

	if err := validate.Required("stream_ok", "body", m.StreamOk); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateStreamReports(formats strfmt.Registry) error {

	if err := validate.Required("stream_reports", "body", m.StreamReports); err != nil {
		return err
	}

	for k := range m.StreamReports {

		if err := validate.Required("stream_reports"+"."+k, "body", m.StreamReports[k]); err != nil {
			return err
		}
		if val, ok := m.StreamReports[k]; ok {
			if err := val.Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("stream_reports" + "." + k)
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("stream_reports" + "." + k)
				}
				return err
			}
		}

	}

	return nil
}

func (m *ExperimentReport) validateStreamRequired(formats strfmt.Registry) error {

	if err := validate.Required("stream_required", "body", m.StreamRequired); err != nil {
		return err
	}

	return nil
}

func (m *ExperimentReport) validateTopicName(formats strfmt.Registry) error {

	if err := validate.Required("topic_name", "body", m.TopicName); err != nil {
		return err
	}

	return nil
}

// ContextValidate validate this experiment report based on the context it is used
func (m *ExperimentReport) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateJumpReport(ctx, formats); err != nil {
		res = append(res, err)
	}

	if err := m.contextValidateStreamReports(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ExperimentReport) contextValidateJumpReport(ctx context.Context, formats strfmt.Registry) error {

	if m.JumpReport != nil {
		if err := m.JumpReport.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("jump_report")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("jump_report")
			}
			return err
		}
	}

	return nil
}

func (m *ExperimentReport) contextValidateStreamReports(ctx context.Context, formats strfmt.Registry) error {

	if err := validate.Required("stream_reports", "body", m.StreamReports); err != nil {
		return err
	}

	for k := range m.StreamReports {

		if val, ok := m.StreamReports[k]; ok {
			if err := val.ContextValidate(ctx, formats); err != nil {
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *ExperimentReport) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ExperimentReport) UnmarshalBinary(b []byte) error {
	var res ExperimentReport
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
