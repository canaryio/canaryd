package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
