/**
 * Copyright 2017 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
package main

import (
	"github.com/Comcast/webpa-common/health"
	"github.com/Comcast/webpa-common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"math"
	"sync"
	"testing"
)

func TestWorkerPool(t *testing.T) {
	assert := assert.New(t)

	workerPool := WorkerPoolFactory{
		NumWorkers: 1,
		QueueSize:  1,
	}.New()

	t.Run("TestWorkerPoolSend", func(t *testing.T) {
		testWG := new(sync.WaitGroup)
		testWG.Add(1)

		require.NotNil(t, workerPool)
		err := workerPool.Send(func(workerID int) {
			testWG.Done()
		})

		testWG.Wait()
		assert.Nil(err)
	})

	workerPool = WorkerPoolFactory{
		NumWorkers: 0,
		QueueSize:  0,
	}.New()

	t.Run("TestWorkerPoolFullQueue", func(t *testing.T) {
		require.NotNil(t, workerPool)
		err := workerPool.Send(func(workerID int) {
			assert.Fail("This should not execute because our worker queue is full and we have no workers.")
		})

		assert.NotNil(err)
	})
}

func TestCaduceusHealth(t *testing.T) {
	assert := assert.New(t)

	testData := []struct {
		inSize       int
		expectedStat health.Stat
	}{
		{inSize: -1, expectedStat: PayloadsOverZero},
		{inSize: 0, expectedStat: PayloadsOverZero},
		{inSize: 99, expectedStat: PayloadsOverZero},
		{inSize: 999, expectedStat: PayloadsOverHundred},
		{inSize: 9999, expectedStat: PayloadsOverThousand},
		{inSize: 10001, expectedStat: PayloadsOverTenThousand},
		{inSize: math.MaxInt32, expectedStat: PayloadsOverTenThousand},
	}

	t.Run("TestIncrementBucket", func(t *testing.T) {
		for _, data := range testData {
			fakeMonitor := new(mockHealthTracker)
			fakeMonitor.On("SendEvent", mock.AnythingOfType("health.HealthFunc")).Run(
				func(args mock.Arguments) {
					healthFunc := args.Get(0).(health.HealthFunc)
					stats := make(health.Stats)

					healthFunc(stats)
					assert.Equal(1, stats[data.expectedStat])
				}).Once()

			caduceusHealth := &CaduceusHealth{fakeMonitor}

			caduceusHealth.IncrementBucket(data.inSize)
			fakeMonitor.AssertExpectations(t)
		}
	})
}

func TestCaduceusHandler(t *testing.T) {
	logger := logging.DefaultLogger()

	fakeSenderWrapper := new(mockSenderWrapper)
	fakeSenderWrapper.On("Queue", mock.AnythingOfType("CaduceusRequest")).Return().Once()

	fakeProfiler := new(mockServerProfiler)

	testHandler := CaduceusHandler{
		handlerProfiler: fakeProfiler,
		senderWrapper:   fakeSenderWrapper,
		Logger:          logger,
	}

	t.Run("TestHandleRequest", func(t *testing.T) {
		testHandler.HandleRequest(0, CaduceusRequest{})

		fakeSenderWrapper.AssertExpectations(t)
		fakeProfiler.AssertExpectations(t)
	})
}
