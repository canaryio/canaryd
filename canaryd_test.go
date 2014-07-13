package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/canaryio/data"
	"github.com/vmihailenco/redis/v2"

	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

type MockRedis struct {
	mock.Mock
}

func (mr *MockRedis) ZAdd(key string, members ...redis.Z) *redis.IntCmd {
	args := mr.Mock.Called(key, members)

	return args.Get(0).(*redis.IntCmd)
}

func (mr *MockRedis) ZRemRangeByScore(key, min, max string) *redis.IntCmd {
	args := mr.Mock.Called(key, min, max)

	return args.Get(0).(*redis.IntCmd)
}

func (mr *MockRedis) ZRevRangeByScore(key string, opt redis.ZRangeByScore) *redis.StringSliceCmd {
	args := mr.Mock.Called(key, opt)

	return args.Get(0).(*redis.StringSliceCmd)
}

func (mr *MockRedis) Publish(channel, message string) *redis.IntCmd {
	args := mr.Mock.Called(channel, message)

	return args.Get(0).(*redis.IntCmd)
}

func TestGetHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(healthHandler))
	defer ts.Close()

	res, err := http.Get(ts.URL + "/health")
	assert.Nil(t, err)

	assert.Equal(t, 200, res.StatusCode)

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	assert.Nil(t, err)

	assert.Equal(t, "OK\n", string(body))
}

func TestRecordMeasurement(t *testing.T) {
	measurement := data.Measurement{
		Check:      data.Check{"FOO", "http://localhost"},
		ID:         "ee338667-42ac-4a4f-49cc-00a6a16a384b",
		Location:   "test-location",
		T:          1403604335,
		ExitStatus: 0,
	}

	s, _ := json.Marshal(measurement)

	mockRedis := new(MockRedis)
	resp := redis.NewIntCmd()

	mockRedis.On(
		"ZAdd",
		"measurements:FOO",
		[]redis.Z{
			redis.Z{
				Score:  float64(measurement.T),
				Member: string(s),
			},
		},
	).Return(resp)

	recorder := NewRecorder(mockRedis, false)

	recorder.record(&measurement)

	mockRedis.AssertExpectations(t)
}

func TestPublishMeasurement(t *testing.T) {
	measurement := data.Measurement{
		Check:      data.Check{"FOO", "http://localhost"},
		ID:         "ee338667-42ac-4a4f-49cc-00a6a16a384b",
		Location:   "test-location",
		T:          1403604335,
		ExitStatus: 0,
	}

	s, _ := json.Marshal(measurement)

	mockRedis := new(MockRedis)
	resp := redis.NewIntCmd()

	mockRedis.On(
		"ZAdd",
		"measurements:FOO",
		[]redis.Z{
			redis.Z{
				Score:  float64(measurement.T),
				Member: string(s),
			},
		},
	).Return(resp)

	mockRedis.On(
		"Publish",
		"measurements:FOO",
		string(s),
	).Return(resp)

	recorder := NewRecorder(mockRedis, true)

	recorder.record(&measurement)

	mockRedis.AssertExpectations(t)
}
