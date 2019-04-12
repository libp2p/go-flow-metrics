package flow

import moved "github.com/libp2p/go-libp2p/metrics"

// Deprecated: Use github.com/libp2p/go-libp2p/metrics.IdleRate instead.
// Warning: it's not possible to alias variables in Go. Setting a value here may have no effect.
var IdleRate = moved.IdleRate

// Deprecated: Use github.com/libp2p/go-libp2p/metrics.Snapshot instead.
type Snapshot = moved.Snapshot

// Deprecated: Use github.com/libp2p/go-libp2p/metrics.Meter instead.
type Meter = moved.Meter

// Deprecated: Use github.com/libp2p/go-libp2p/metrics.MeterRegistry instead.
type MeterRegistry = moved.MeterRegistry

