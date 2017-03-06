package main

import (
	"bytes"
	"github.com/Comcast/webpa-common/health"
	"github.com/Comcast/webpa-common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Begin mock declarations
type mockHandler struct {
	mock.Mock
}

func (m *mockHandler) HandleRequest(workerID int, inRequest CaduceusRequest) {
	return
}

type mockHealthTracker struct {
	mock.Mock
}

func (m *mockHealthTracker) Increment(inStat health.Stat) {
	return
}

func (m *mockHealthTracker) IncrementBucket(inSize int) {
	return
}

// Begin test functions
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestServeHTTP(t *testing.T) {
	assert := assert.New(t)

	logger := logging.DefaultLogger()
	fakeHandler := &mockHandler{}
	fakeHealth := &mockHealthTracker{}

	serverWrapper := &ServerHandler{
		logger:          logger,
		caduceusHandler: fakeHandler,
		caduceusHealth:  fakeHealth,
		workerPool: WorkerPoolFactory{
			NumWorkers: 1,
			QueueSize:  1,
		}.New(),
	}

	testServer := httptest.NewServer(serverWrapper)
	defer testServer.Close()

	buf := bytes.NewBufferString("Test message.")

	res, err := http.Post(testServer.URL, "text/plain", buf)
	assert.Nil(err)
	defer res.Body.Close()

	resMsg, err := ioutil.ReadAll(res.Body)
	assert.Nil(err)
	assert.Equal("Request placed on to queue.\n", string(resMsg))

	fakeHandler.AssertExpectations(t)
	fakeHealth.AssertExpectations(t)
}

// func TestHandleRequest(t *testing.T) {
// 	assert := assert.New(t)
//
// 	logger := logging.DefaultLogger()
// 	testHandler := CaduceusHandler{
// 		logger: logger,
// 	}
// 	testRequest := CaduceusRequest{
// 		Payload:     []byte("Test payload."),
// 		ContentType: "text/plain",
// 		TargetURL:   "mytesturl",
// 		Timestamps: CaduceusTimestamps{
// 			TimeReceived:        0,
// 			TimeAccepted:        1,
// 			TimeProcessingStart: 2,
// 			TimeProcessingEnd:   3,
// 		},
// 	}
//
// 	testHandler.HandleRequest(0, testRequest)
// }
