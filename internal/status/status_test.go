package status

import (
	"testing"
	"time"

	jc "github.com/practable/jump/pkg/status"
	rc "github.com/practable/relay/pkg/status"
	"github.com/stretchr/testify/assert"
)

func TestUpdateFromJump(t *testing.T) {

	s := New()

	r := []jc.Report{
		jc.Report{
			Topic:      "foo/000",
			RemoteAddr: "192.1.1.1",
		},
		jc.Report{
			Topic:      "foo",
			RemoteAddr: " 135.9.9.9"}, //leading space
		jc.Report{
			Topic:      "bar",
			RemoteAddr: "135.9.9.12 "}, //trailing space
	}

	s.updateFromJump(r)

	expected := make(map[string]string)
	expected["foo"] = "135.9.9.9"
	expected["bar"] = "135.9.9.12"

	assert.Equal(t, expected, s.IPAddress)

}
func TestGetID(t *testing.T) {

	id, err := getID("pend00-st-data")
	assert.NoError(t, err)
	assert.Equal(t, "pend00", id)

	id, err = getID("")

	assert.Error(t, err)
	assert.Equal(t, "", id)
}

func TestGetStream(t *testing.T) {

	stream, err := getStream("pend00-st-data")
	assert.NoError(t, err)
	assert.Equal(t, "data", stream)

	stream, err = getStream("")

	assert.Error(t, err)
	assert.Equal(t, "", stream)

}

func TestUpdateFromRelay(t *testing.T) {

	// setup
	s := New()

	j := []jc.Report{
		jc.Report{
			Topic:      "foo/000",
			RemoteAddr: "192.1.1.1",
		},
		jc.Report{
			Topic:      "foo",
			RemoteAddr: "135.9.9.9"},
		jc.Report{
			Topic:      "bar",
			RemoteAddr: "135.9.9.12"},
	}

	s.updateFromJump(j)

	expected := make(map[string]string)
	expected["foo"] = "135.9.9.9"
	expected["bar"] = "135.9.9.12"

	assert.Equal(t, expected, s.IPAddress)

	// test

	r := []rc.Report{
		rc.Report{
			Topic:      "foo-st-data",
			RemoteAddr: "192.1.1.1", //client
			Stats: rc.RxTx{
				Tx: rc.Statistics{
					Last:  time.Duration(time.Hour),
					Never: false,
				},
			},
		},
		rc.Report{
			Topic:      "foo-st-data",
			RemoteAddr: "135.9.9.9",
			Stats: rc.RxTx{
				Tx: rc.Statistics{
					Last:  time.Duration(20 * time.Second),
					Never: false,
				},
			},
		},
		rc.Report{
			Topic:      "foo-st-video",
			RemoteAddr: "135.9.9.9",
			Stats: rc.RxTx{
				Tx: rc.Statistics{
					Never: true,
				},
			},
		},
		rc.Report{
			Topic:      "bar-st-video",
			RemoteAddr: "135.9.9.12",
			Stats: rc.RxTx{
				Tx: rc.Statistics{
					Last:  time.Duration(5 * time.Second),
					Never: false,
				},
			},
		},
		rc.Report{
			Topic:      "bar-st-data",
			RemoteAddr: "135.9.9.12",
			Stats: rc.RxTx{
				Tx: rc.Statistics{
					Last:  time.Duration(2 * time.Second),
					Never: false,
				},
			},
		},
	}

	config := Config{
		HealthyDuration: time.Duration(10 * time.Second),
	}

	s.updateFromRelay(r, config)

	// TODO add check of overall healthy status when this is implemented

	expected2 := make(map[string]Report)

	expected2["foo"] = Report{}

	expected2["bar"] = Report{Healthy: true, Streams: map[string]bool{"data": true, "video": true}, Reports: map[string]rc.Report{"data": rc.Report{Topic: "bar-st-data", CanRead: false, CanWrite: false, Connected: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), RemoteAddr: "135.9.9.12", UserAgent: "", Stats: rc.RxTx{Tx: rc.Statistics{Last: 2000000000, Size: 0, FPS: 0, Never: false}, Rx: rc.Statistics{Last: 0, Size: 0, FPS: 0, Never: false}}}, "video": rc.Report{Topic: "bar-st-video", CanRead: false, CanWrite: false, Connected: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), RemoteAddr: "135.9.9.12", UserAgent: "", Stats: rc.RxTx{Tx: rc.Statistics{Last: 5000000000, Size: 0, FPS: 0, Never: false}, Rx: rc.Statistics{Last: 0, Size: 0, FPS: 0, Never: false}}}}}

	expected2["foo"] = Report{Healthy: false, Streams: map[string]bool{"data": false, "video": false}, Reports: map[string]rc.Report{"data": rc.Report{Topic: "foo-st-data", CanRead: false, CanWrite: false, Connected: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), RemoteAddr: "135.9.9.9", UserAgent: "", Stats: rc.RxTx{Tx: rc.Statistics{Last: 20000000000, Size: 0, FPS: 0, Never: false}, Rx: rc.Statistics{Last: 0, Size: 0, FPS: 0, Never: false}}}, "video": rc.Report{Topic: "foo-st-video", CanRead: false, CanWrite: false, Connected: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), RemoteAddr: "135.9.9.9", UserAgent: "", Stats: rc.RxTx{Tx: rc.Statistics{Last: 0, Size: 0, FPS: 0, Never: true}, Rx: rc.Statistics{Last: 0, Size: 0, FPS: 0, Never: false}}}}}

	assert.Equal(t, expected2, s.Experiment)

}
