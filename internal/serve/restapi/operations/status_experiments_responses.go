// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/practable/status/internal/serve/models"
)

// StatusExperimentsOKCode is the HTTP code returned for type StatusExperimentsOK
const StatusExperimentsOKCode int = 200

/*StatusExperimentsOK OK

swagger:response statusExperimentsOK
*/
type StatusExperimentsOK struct {

	/*
	  In: Body
	*/
	Payload models.ExperimentReports `json:"body,omitempty"`
}

// NewStatusExperimentsOK creates StatusExperimentsOK with default headers values
func NewStatusExperimentsOK() *StatusExperimentsOK {

	return &StatusExperimentsOK{}
}

// WithPayload adds the payload to the status experiments o k response
func (o *StatusExperimentsOK) WithPayload(payload models.ExperimentReports) *StatusExperimentsOK {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the status experiments o k response
func (o *StatusExperimentsOK) SetPayload(payload models.ExperimentReports) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *StatusExperimentsOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(200)
	payload := o.Payload
	if payload == nil {
		// return empty array
		payload = models.ExperimentReports{}
	}

	if err := producer.Produce(rw, payload); err != nil {
		panic(err) // let the recovery middleware deal with this
	}
}

// StatusExperimentsUnauthorizedCode is the HTTP code returned for type StatusExperimentsUnauthorized
const StatusExperimentsUnauthorizedCode int = 401

/*StatusExperimentsUnauthorized Unauthorized

swagger:response statusExperimentsUnauthorized
*/
type StatusExperimentsUnauthorized struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewStatusExperimentsUnauthorized creates StatusExperimentsUnauthorized with default headers values
func NewStatusExperimentsUnauthorized() *StatusExperimentsUnauthorized {

	return &StatusExperimentsUnauthorized{}
}

// WithPayload adds the payload to the status experiments unauthorized response
func (o *StatusExperimentsUnauthorized) WithPayload(payload *models.Error) *StatusExperimentsUnauthorized {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the status experiments unauthorized response
func (o *StatusExperimentsUnauthorized) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *StatusExperimentsUnauthorized) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(401)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// StatusExperimentsNotFoundCode is the HTTP code returned for type StatusExperimentsNotFound
const StatusExperimentsNotFoundCode int = 404

/*StatusExperimentsNotFound The specified resource was not found

swagger:response statusExperimentsNotFound
*/
type StatusExperimentsNotFound struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewStatusExperimentsNotFound creates StatusExperimentsNotFound with default headers values
func NewStatusExperimentsNotFound() *StatusExperimentsNotFound {

	return &StatusExperimentsNotFound{}
}

// WithPayload adds the payload to the status experiments not found response
func (o *StatusExperimentsNotFound) WithPayload(payload *models.Error) *StatusExperimentsNotFound {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the status experiments not found response
func (o *StatusExperimentsNotFound) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *StatusExperimentsNotFound) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(404)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// StatusExperimentsInternalServerErrorCode is the HTTP code returned for type StatusExperimentsInternalServerError
const StatusExperimentsInternalServerErrorCode int = 500

/*StatusExperimentsInternalServerError Internal Error

swagger:response statusExperimentsInternalServerError
*/
type StatusExperimentsInternalServerError struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewStatusExperimentsInternalServerError creates StatusExperimentsInternalServerError with default headers values
func NewStatusExperimentsInternalServerError() *StatusExperimentsInternalServerError {

	return &StatusExperimentsInternalServerError{}
}

// WithPayload adds the payload to the status experiments internal server error response
func (o *StatusExperimentsInternalServerError) WithPayload(payload *models.Error) *StatusExperimentsInternalServerError {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the status experiments internal server error response
func (o *StatusExperimentsInternalServerError) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *StatusExperimentsInternalServerError) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(500)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}
