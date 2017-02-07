// includes code from
// https://raw.githubusercontent.com/rcrowley/go-metrics/master/sample.go
// Copyright 2012 Richard Crowley. All rights reserved.

package metrics

import "sync/atomic"

// Gauges hold an int64 value that can be set arbitrarily.
type Gauge interface {
	Metric

	Update(int64)
	Value() int64
}

func NewGauge(meta *MetricMeta) Gauge {
	if UseNilMetrics {
		return NilGauge{}
	}
	return &StandardGauge{
		MetricMeta: meta,
		value:      0,
	}
}

func RegGauge(name string, tagStrings ...string) Gauge {
	tr := NewGauge(NewMetricMeta(name, tagStrings))
	MetricStats.Register(tr)
	return tr
}

// GaugeSnapshot is a read-only copy of another Gauge.
type GaugeSnapshot struct {
	value int64
	*MetricMeta
}

// Snapshot returns the snapshot.
func (g GaugeSnapshot) Snapshot() Metric { return g }

// Update panics.
func (GaugeSnapshot) Update(int64) {
	panic("Update called on a GaugeSnapshot")
}

// Value returns the value at the time the snapshot was taken.
func (g GaugeSnapshot) Value() int64 { return g.value }

// NilGauge is a no-op Gauge.
type NilGauge struct{ *MetricMeta }

// Snapshot is a no-op.
func (NilGauge) Snapshot() Metric { return NilGauge{} }

// Update is a no-op.
func (NilGauge) Update(v int64) {}

// Value is a no-op.
func (NilGauge) Value() int64 { return 0 }

// StandardGauge is the standard implementation of a Gauge and uses the
// sync/atomic package to manage a single int64 value.
// atomic needs 64-bit aligned memory which is ensure for first word
type StandardGauge struct {
	value int64
	*MetricMeta
}

// Snapshot returns a read-only copy of the gauge.
func (g *StandardGauge) Snapshot() Metric {
	return GaugeSnapshot{MetricMeta: g.MetricMeta, value: g.value}
}

// Update updates the gauge's value.
func (g *StandardGauge) Update(v int64) {
	atomic.StoreInt64(&g.value, v)
}

// Value returns the gauge's current value.
func (g *StandardGauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}
