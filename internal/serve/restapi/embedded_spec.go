// Code generated by go-swagger; DO NOT EDIT.

package restapi

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
)

var (
	// SwaggerJSON embedded version of the swagger document used at generation time
	SwaggerJSON json.RawMessage
	// FlatSwaggerJSON embedded flattened version of the swagger document used at generation time
	FlatSwaggerJSON json.RawMessage
)

func init() {
	SwaggerJSON = json.RawMessage([]byte(`{
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "description": "API for getting the status of experiments",
    "title": "Status",
    "contact": {
      "name": "Timothy Drysdale",
      "url": "https://github.com/timdrysdale",
      "email": "timothy.d.drysdale@gmail.com"
    },
    "version": "0.1"
  },
  "host": "localhost",
  "basePath": "/api/v1",
  "paths": {
    "/experiments": {
      "get": {
        "description": "Get the status of all experiments",
        "produces": [
          "application/json"
        ],
        "summary": "Get the status of all experiments",
        "operationId": "statusExperiments",
        "responses": {
          "200": {
            "description": "OK",
            "schema": {
              "$ref": "#/definitions/ExperimentReports"
            }
          },
          "401": {
            "$ref": "#/responses/Unauthorized"
          },
          "404": {
            "$ref": "#/responses/NotFound"
          },
          "500": {
            "$ref": "#/responses/InternalError"
          }
        }
      }
    },
    "/experiments/events/{name}": {
      "get": {
        "description": "Get a list of the health events recorded for an experiment",
        "produces": [
          "application/json"
        ],
        "summary": "Get the health events for an experiment",
        "operationId": "healthEvents",
        "parameters": [
          {
            "type": "string",
            "description": "topic_stub of the experiment e.g pend00 (not r-pend00)",
            "name": "name",
            "in": "path",
            "required": true
          }
        ],
        "responses": {
          "200": {
            "description": "OK",
            "schema": {
              "$ref": "#/definitions/HealthEvents"
            }
          },
          "401": {
            "$ref": "#/responses/Unauthorized"
          },
          "404": {
            "$ref": "#/responses/NotFound"
          },
          "500": {
            "$ref": "#/responses/InternalError"
          }
        }
      }
    }
  },
  "definitions": {
    "Error": {
      "type": "object",
      "required": [
        "code",
        "message"
      ],
      "properties": {
        "code": {
          "type": "string"
        },
        "message": {
          "type": "string"
        }
      }
    },
    "ExperimentReport": {
      "type": "object",
      "title": "Report on the Status of an experiment",
      "required": [
        "available",
        "first_checked",
        "healthy",
        "jump_ok",
        "last_checked_jump",
        "last_checked_streams",
        "last_found_in_manifest",
        "resource_name",
        "stream_ok",
        "stream_reports",
        "stream_required",
        "topic_name"
      ],
      "properties": {
        "available": {
          "description": "is it set as available on the booking system?",
          "type": "boolean"
        },
        "first_checked": {
          "type": "string"
        },
        "health_events": {
          "description": "number of health events recorded",
          "type": "integer"
        },
        "healthy": {
          "type": "boolean"
        },
        "jump_ok": {
          "type": "boolean"
        },
        "jump_report": {
          "$ref": "#/definitions/JumpReport"
        },
        "last_checked_jump": {
          "type": "string"
        },
        "last_checked_streams": {
          "type": "string"
        },
        "last_found_in_manifest": {
          "type": "string"
        },
        "resource_name": {
          "description": "name of the resouce in the manifest",
          "type": "string",
          "example": "r-spin30"
        },
        "stream_ok": {
          "type": "object",
          "additionalProperties": {
            "type": "boolean"
          }
        },
        "stream_reports": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/definitions/StreamReport"
          }
        },
        "stream_required": {
          "description": "defaults to true for required stream, false currently undefined, but kept as a map for consistenct with stream_reports and stream_ok",
          "type": "object",
          "additionalProperties": {
            "type": "boolean"
          }
        },
        "topic_name": {
          "description": "topic stub in stream names",
          "type": "string",
          "example": "spin30"
        }
      }
    },
    "ExperimentReports": {
      "type": "array",
      "title": "List of experiment reports",
      "items": {
        "$ref": "#/definitions/ExperimentReport"
      }
    },
    "HealthEvent": {
      "description": "information on what streams are available when an experiment changes health status",
      "title": "health event",
      "properties": {
        "healthy": {
          "description": "is experiment healthy?",
          "type": "boolean"
        },
        "issues": {
          "description": "list of issues, if any",
          "type": "array",
          "items": {
            "description": "individual issue",
            "type": "string"
          }
        },
        "jump_ok": {
          "description": "is the jump connection ok?",
          "type": "boolean"
        },
        "stream_ok": {
          "description": "which streams are connected, and are they ok? Name of stream in key",
          "type": "object",
          "additionalProperties": {
            "type": "boolean"
          }
        },
        "when": {
          "description": "the time and date of the event occurring",
          "type": "string"
        }
      }
    },
    "HealthEvents": {
      "description": "list of health events",
      "type": "array",
      "title": "Health events",
      "items": {
        "$ref": "#/definitions/HealthEvent"
      }
    },
    "JumpReport": {
      "type": "object",
      "title": "Status of the jump connection for an experiment",
      "required": [
        "connected",
        "expires_at",
        "scopes",
        "stats",
        "topic",
        "user_agent"
      ],
      "properties": {
        "can_read": {
          "type": "boolean"
        },
        "can_write": {
          "type": "boolean"
        },
        "connected": {
          "description": "date and time connection made",
          "type": "string"
        },
        "expires_at": {
          "description": "expiry date and time in the token used to authenticate the connection",
          "type": "string"
        },
        "remote_addr": {
          "description": "list of IP addresses for client (typically \u003cclient\u003e, \u003cproxy 1\u003e, etc)",
          "type": "string"
        },
        "scopes": {
          "description": "list of scopes supplied in token used to authenticate the connection",
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "stats": {
          "$ref": "#/definitions/RxTx"
        },
        "topic": {
          "description": "topic_stub for experiment e.g. pend00",
          "type": "string"
        },
        "user_agent": {
          "description": "what tool is user using to connect",
          "type": "string"
        }
      }
    },
    "RxTx": {
      "description": "receive and transmit statistics for a connection",
      "type": "object",
      "properties": {
        "rx": {
          "$ref": "#/definitions/Statistics"
        },
        "tx": {
          "$ref": "#/definitions/Statistics"
        }
      }
    },
    "Statistics": {
      "description": "connection statistics",
      "type": "object",
      "properties": {
        "fps": {
          "description": "messages per second (frames per second if video)",
          "type": "integer"
        },
        "last": {
          "description": "date and time of the last message sent",
          "type": "string"
        },
        "never": {
          "description": "true if not messages ever sent on this connection",
          "type": "boolean"
        },
        "size": {
          "description": "size in bytes of the last message sent",
          "type": "integer"
        }
      }
    },
    "StreamReport": {
      "type": "object",
      "title": "Status of a stream",
      "required": [
        "connected",
        "expires_at",
        "scopes",
        "stats",
        "topic",
        "user_agent"
      ],
      "properties": {
        "can_read": {
          "type": "boolean"
        },
        "can_write": {
          "type": "boolean"
        },
        "connected": {
          "description": "date and time connection made",
          "type": "string"
        },
        "expires_at": {
          "description": "expiry date and time in the token used to authenticate the connection",
          "type": "string"
        },
        "remote_addr": {
          "description": "list of IP addresses for client (typically \u003cclient\u003e, \u003cproxy 1\u003e, etc)",
          "type": "string"
        },
        "scopes": {
          "description": "list of scopes supplied in token used to authenticate the connection",
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "stats": {
          "$ref": "#/definitions/RxTx"
        },
        "topic": {
          "description": "topic_stub for experiment e.g. pend00",
          "type": "string"
        },
        "user_agent": {
          "description": "what tool is user using to connect",
          "type": "string"
        }
      }
    }
  },
  "responses": {
    "InternalError": {
      "description": "Internal Error",
      "schema": {
        "$ref": "#/definitions/Error"
      }
    },
    "NotFound": {
      "description": "The specified resource was not found",
      "schema": {
        "$ref": "#/definitions/Error"
      }
    },
    "Unauthorized": {
      "description": "Unauthorized",
      "schema": {
        "$ref": "#/definitions/Error"
      }
    }
  },
  "securityDefinitions": {
    "Bearer": {
      "type": "apiKey",
      "name": "Authorization",
      "in": "header"
    }
  }
}`))
	FlatSwaggerJSON = json.RawMessage([]byte(`{
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "description": "API for getting the status of experiments",
    "title": "Status",
    "contact": {
      "name": "Timothy Drysdale",
      "url": "https://github.com/timdrysdale",
      "email": "timothy.d.drysdale@gmail.com"
    },
    "version": "0.1"
  },
  "host": "localhost",
  "basePath": "/api/v1",
  "paths": {
    "/experiments": {
      "get": {
        "description": "Get the status of all experiments",
        "produces": [
          "application/json"
        ],
        "summary": "Get the status of all experiments",
        "operationId": "statusExperiments",
        "responses": {
          "200": {
            "description": "OK",
            "schema": {
              "$ref": "#/definitions/ExperimentReports"
            }
          },
          "401": {
            "description": "Unauthorized",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          },
          "404": {
            "description": "The specified resource was not found",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          },
          "500": {
            "description": "Internal Error",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          }
        }
      }
    },
    "/experiments/events/{name}": {
      "get": {
        "description": "Get a list of the health events recorded for an experiment",
        "produces": [
          "application/json"
        ],
        "summary": "Get the health events for an experiment",
        "operationId": "healthEvents",
        "parameters": [
          {
            "type": "string",
            "description": "topic_stub of the experiment e.g pend00 (not r-pend00)",
            "name": "name",
            "in": "path",
            "required": true
          }
        ],
        "responses": {
          "200": {
            "description": "OK",
            "schema": {
              "$ref": "#/definitions/HealthEvents"
            }
          },
          "401": {
            "description": "Unauthorized",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          },
          "404": {
            "description": "The specified resource was not found",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          },
          "500": {
            "description": "Internal Error",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          }
        }
      }
    }
  },
  "definitions": {
    "Error": {
      "type": "object",
      "required": [
        "code",
        "message"
      ],
      "properties": {
        "code": {
          "type": "string"
        },
        "message": {
          "type": "string"
        }
      }
    },
    "ExperimentReport": {
      "type": "object",
      "title": "Report on the Status of an experiment",
      "required": [
        "available",
        "first_checked",
        "healthy",
        "jump_ok",
        "last_checked_jump",
        "last_checked_streams",
        "last_found_in_manifest",
        "resource_name",
        "stream_ok",
        "stream_reports",
        "stream_required",
        "topic_name"
      ],
      "properties": {
        "available": {
          "description": "is it set as available on the booking system?",
          "type": "boolean"
        },
        "first_checked": {
          "type": "string"
        },
        "health_events": {
          "description": "number of health events recorded",
          "type": "integer"
        },
        "healthy": {
          "type": "boolean"
        },
        "jump_ok": {
          "type": "boolean"
        },
        "jump_report": {
          "$ref": "#/definitions/JumpReport"
        },
        "last_checked_jump": {
          "type": "string"
        },
        "last_checked_streams": {
          "type": "string"
        },
        "last_found_in_manifest": {
          "type": "string"
        },
        "resource_name": {
          "description": "name of the resouce in the manifest",
          "type": "string",
          "example": "r-spin30"
        },
        "stream_ok": {
          "type": "object",
          "additionalProperties": {
            "type": "boolean"
          }
        },
        "stream_reports": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/definitions/StreamReport"
          }
        },
        "stream_required": {
          "description": "defaults to true for required stream, false currently undefined, but kept as a map for consistenct with stream_reports and stream_ok",
          "type": "object",
          "additionalProperties": {
            "type": "boolean"
          }
        },
        "topic_name": {
          "description": "topic stub in stream names",
          "type": "string",
          "example": "spin30"
        }
      }
    },
    "ExperimentReports": {
      "type": "array",
      "title": "List of experiment reports",
      "items": {
        "$ref": "#/definitions/ExperimentReport"
      }
    },
    "HealthEvent": {
      "description": "information on what streams are available when an experiment changes health status",
      "title": "health event",
      "properties": {
        "healthy": {
          "description": "is experiment healthy?",
          "type": "boolean"
        },
        "issues": {
          "description": "list of issues, if any",
          "type": "array",
          "items": {
            "description": "individual issue",
            "type": "string"
          }
        },
        "jump_ok": {
          "description": "is the jump connection ok?",
          "type": "boolean"
        },
        "stream_ok": {
          "description": "which streams are connected, and are they ok? Name of stream in key",
          "type": "object",
          "additionalProperties": {
            "type": "boolean"
          }
        },
        "when": {
          "description": "the time and date of the event occurring",
          "type": "string"
        }
      }
    },
    "HealthEvents": {
      "description": "list of health events",
      "type": "array",
      "title": "Health events",
      "items": {
        "$ref": "#/definitions/HealthEvent"
      }
    },
    "JumpReport": {
      "type": "object",
      "title": "Status of the jump connection for an experiment",
      "required": [
        "connected",
        "expires_at",
        "scopes",
        "stats",
        "topic",
        "user_agent"
      ],
      "properties": {
        "can_read": {
          "type": "boolean"
        },
        "can_write": {
          "type": "boolean"
        },
        "connected": {
          "description": "date and time connection made",
          "type": "string"
        },
        "expires_at": {
          "description": "expiry date and time in the token used to authenticate the connection",
          "type": "string"
        },
        "remote_addr": {
          "description": "list of IP addresses for client (typically \u003cclient\u003e, \u003cproxy 1\u003e, etc)",
          "type": "string"
        },
        "scopes": {
          "description": "list of scopes supplied in token used to authenticate the connection",
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "stats": {
          "$ref": "#/definitions/RxTx"
        },
        "topic": {
          "description": "topic_stub for experiment e.g. pend00",
          "type": "string"
        },
        "user_agent": {
          "description": "what tool is user using to connect",
          "type": "string"
        }
      }
    },
    "RxTx": {
      "description": "receive and transmit statistics for a connection",
      "type": "object",
      "properties": {
        "rx": {
          "$ref": "#/definitions/Statistics"
        },
        "tx": {
          "$ref": "#/definitions/Statistics"
        }
      }
    },
    "Statistics": {
      "description": "connection statistics",
      "type": "object",
      "properties": {
        "fps": {
          "description": "messages per second (frames per second if video)",
          "type": "integer"
        },
        "last": {
          "description": "date and time of the last message sent",
          "type": "string"
        },
        "never": {
          "description": "true if not messages ever sent on this connection",
          "type": "boolean"
        },
        "size": {
          "description": "size in bytes of the last message sent",
          "type": "integer"
        }
      }
    },
    "StreamReport": {
      "type": "object",
      "title": "Status of a stream",
      "required": [
        "connected",
        "expires_at",
        "scopes",
        "stats",
        "topic",
        "user_agent"
      ],
      "properties": {
        "can_read": {
          "type": "boolean"
        },
        "can_write": {
          "type": "boolean"
        },
        "connected": {
          "description": "date and time connection made",
          "type": "string"
        },
        "expires_at": {
          "description": "expiry date and time in the token used to authenticate the connection",
          "type": "string"
        },
        "remote_addr": {
          "description": "list of IP addresses for client (typically \u003cclient\u003e, \u003cproxy 1\u003e, etc)",
          "type": "string"
        },
        "scopes": {
          "description": "list of scopes supplied in token used to authenticate the connection",
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "stats": {
          "$ref": "#/definitions/RxTx"
        },
        "topic": {
          "description": "topic_stub for experiment e.g. pend00",
          "type": "string"
        },
        "user_agent": {
          "description": "what tool is user using to connect",
          "type": "string"
        }
      }
    }
  },
  "responses": {
    "InternalError": {
      "description": "Internal Error",
      "schema": {
        "$ref": "#/definitions/Error"
      }
    },
    "NotFound": {
      "description": "The specified resource was not found",
      "schema": {
        "$ref": "#/definitions/Error"
      }
    },
    "Unauthorized": {
      "description": "Unauthorized",
      "schema": {
        "$ref": "#/definitions/Error"
      }
    }
  },
  "securityDefinitions": {
    "Bearer": {
      "type": "apiKey",
      "name": "Authorization",
      "in": "header"
    }
  }
}`))
}
