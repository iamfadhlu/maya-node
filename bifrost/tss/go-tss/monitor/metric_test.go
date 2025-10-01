package monitor

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func getCounterValue(metric *prometheus.CounterVec, flag string) (float64, error) {
	m := &dto.Metric{}
	if err := metric.WithLabelValues(flag).Write(m); err != nil {
		return 0, err
	}
	return m.Counter.GetValue(), nil
}

func TestMetric_UpdateKeySign(t *testing.T) {
	metrics := NewMetric()
	testTime := time.Second
	metrics.UpdateKeySign(testTime, true)
	metrics.UpdateKeySign(testTime, true)
	metrics.UpdateKeySign(testTime, true)

	metrics.UpdateKeySign(testTime, false)
	metrics.UpdateKeySign(testTime, false)
	metrics.UpdateKeySign(testTime, false)
	metrics.UpdateKeySign(testTime, false)
	metrics.UpdateKeySign(testTime, false)

	val, err := getCounterValue(metrics.keysignCounter, "success")
	assert.Nil(t, err)
	assert.Equal(t, float64(3), val)
	val, err = getCounterValue(metrics.keysignCounter, "failure")
	assert.Nil(t, err)
	assert.Equal(t, float64(5), val)

	m := &dto.Metric{}
	err = metrics.keySignTime.Write(m)
	assert.Nil(t, err)
	val = m.Gauge.GetValue()
	assert.Equal(t, float64(time.Second), val)
}

func TestMetric_UpdateKeyGen(t *testing.T) {
	metrics := NewMetric()
	testTime := time.Second
	metrics.UpdateKeyGen(testTime, true)
	metrics.UpdateKeyGen(testTime, true)
	metrics.UpdateKeyGen(testTime, true)

	metrics.UpdateKeyGen(testTime, false)
	metrics.UpdateKeyGen(testTime, false)
	metrics.UpdateKeyGen(testTime, false)
	metrics.UpdateKeyGen(testTime, false)
	metrics.UpdateKeyGen(testTime, false)

	val, err := getCounterValue(metrics.keygenCounter, "success")
	assert.Nil(t, err)
	assert.Equal(t, float64(3), val)
	val, err = getCounterValue(metrics.keygenCounter, "failure")
	assert.Nil(t, err)
	assert.Equal(t, float64(5), val)
}

func TestMetric_ErrorConditions(t *testing.T) {
	metrics := NewMetric()

	// Test with zero duration
	metrics.UpdateKeySign(0, true)

	// Test with negative duration (if applicable)
	metrics.UpdateKeySign(-time.Second, false)

	// Verify metrics still work correctly
	val, err := getCounterValue(metrics.keysignCounter, "success")
	assert.Nil(t, err)
	assert.Equal(t, float64(1), val)
}

func TestMetric_ConcurrentUpdates(t *testing.T) {
	metrics := NewMetric()

	// Test concurrent updates
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.UpdateKeySign(time.Millisecond, true)
		}()
	}
	wg.Wait()

	val, err := getCounterValue(metrics.keysignCounter, "success")
	assert.Nil(t, err)
	assert.Equal(t, float64(10), val)
}
