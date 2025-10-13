/*
 *     Copyright 2025 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package evaluator

import (
	"sort"
	"strings"

	"d7y.io/dragonfly/v2/pkg/types"
	"d7y.io/dragonfly/v2/scheduler/resource/persistentcache"
	"d7y.io/dragonfly/v2/scheduler/resource/standard"
)

const (
	// defaultLoadQualityWeight is the weight of load quality.
	defaultLoadQualityWeight = 0.6

	// defaultHostTypeWeight is the weight of host type.
	defaultIDCAffinityWeight = 0.2

	// defaultLocationAffinityWeight is the weight of location affinity.
	defaultLocationAffinityWeight = 0.1

	// defaultHostTypeWeight is the weight of host type.
	defaultHostTypeWeight = 0.1
)

const (
	// defaultPeakBandwidthUsageWeight is the weight of peak bandwidth usage.
	defaultPeakBandwidthUsageWeight = 0.5

	// defaultBandwidthDurationWeight is the weight of bandwidth duration.
	defaultBandwidthDurationWeight = 0.3

	// defaultConcurrencyWeight is the weight of concurrency.
	defaultConcurrencyWeight = 0.2
)

// evaluatorDefault is an implementation of Evaluator.
type evaluatorDefault struct {
	evaluator
}

// newEvaluatorDefault returns a new EvaluatorDefault.
func newEvaluatorDefault() Evaluator {
	return &evaluatorDefault{}
}

// EvaluateParents sort parents by evaluating multiple feature scores.
func (e *evaluatorDefault) EvaluateParents(parents []*standard.Peer, child *standard.Peer) []*standard.Peer {
	sort.Slice(
		parents,
		func(i, j int) bool {
			return e.evaluateParents(parents[i], child) > e.evaluateParents(parents[j], child)
		},
	)

	return parents
}

// evaluateParents sort parents by evaluating multiple feature scores.
func (e *evaluatorDefault) evaluateParents(parent *standard.Peer, child *standard.Peer) float64 {
	return defaultLoadQualityWeight*e.calculateLoadQualityScore(parent, child) +
		defaultIDCAffinityWeight*e.calculateIDCAffinityScore(parent.Host.Network.IDC, child.Host.Network.IDC) +
		defaultLocationAffinityWeight*e.calculateLocationAffinityScore(parent.Host.Network.Location, child.Host.Network.Location) +
		defaultHostTypeWeight*e.calculateHostTypeScore(parent)
}

// calculateLoadQualityScore 0.0~1.0 larger and better.
func (e *evaluatorDefault) calculateLoadQualityScore(parent *standard.Peer, child *standard.Peer) float64 {
	return defaultPeakBandwidthUsageWeight*e.calculatePeakBandwidthUsageScore(parent) +
		defaultBandwidthDurationWeight*e.calculateBandwidthDurationScore(parent, child) +
		defaultConcurrencyWeight*e.calculateConcurrencyScore(parent, child)
}

// calculatePeakBandwidthUsageScore 0.0~1.0 larger and better.
func (e *evaluatorDefault) calculatePeakBandwidthUsageScore(parent *standard.Peer) float64 {
	maxTxBandwidth := parent.Host.Network.MaxTxBandwidth
	if maxTxBandwidth == 0 {
		return minScore
	}

	txBandwidth := parent.Host.TxBandwidth.Load()
	if txBandwidth >= maxTxBandwidth {
		return minScore
	}

	return 1 - float64(txBandwidth)/float64(maxTxBandwidth)
}

// calculateBandwidthDurationScore 0.0~1.0 larger and better.
func (e *evaluatorDefault) calculateBandwidthDurationScore(parent *standard.Peer, child *standard.Peer) float64 {
	return minScore
}

// calculateConcurrencyScore 0.0~1.0 larger and better.
func (e *evaluatorDefault) calculateConcurrencyScore(parent *standard.Peer, child *standard.Peer) float64 {
	return minScore
}

// EvaluatePersistentCacheParents sort persistent cache parents by evaluating multiple feature scores.
func (e *evaluatorDefault) EvaluatePersistentCacheParents(parents []*persistentcache.Peer, child *persistentcache.Peer) []*persistentcache.Peer {
	sort.Slice(
		parents,
		func(i, j int) bool {
			return e.evaluatePersistentCacheParents(parents[i], child) > e.evaluatePersistentCacheParents(parents[j], child)
		},
	)

	return parents
}

// evaluatePersistentCacheParents sort persistent cache parents by evaluating multiple feature scores.
func (e *evaluatorDefault) evaluatePersistentCacheParents(parent *persistentcache.Peer, child *persistentcache.Peer) float64 {
	return defaultIDCAffinityWeight*e.calculateIDCAffinityScore(parent.Host.Network.IDC, child.Host.Network.IDC) +
		defaultLocationAffinityWeight*e.calculateLocationAffinityScore(parent.Host.Network.Location, child.Host.Network.Location)
}

// calculateIDCAffinityScore 0.0~1.0 larger and better.
func (e *evaluatorDefault) calculateIDCAffinityScore(dst, src string) float64 {
	if dst == "" || src == "" {
		return minScore
	}

	if strings.EqualFold(dst, src) {
		return maxScore
	}

	return minScore
}

// calculateMultiElementAffinityScore 0.0~1.0 larger and better.
func (e *evaluatorDefault) calculateLocationAffinityScore(dst, src string) float64 {
	if dst == "" || src == "" {
		return minScore
	}

	if strings.EqualFold(dst, src) {
		return maxScore
	}

	// Calculate the number of multi-element matches divided by "|".
	var score, elementLen int
	dstElements := strings.Split(dst, types.AffinitySeparator)
	srcElements := strings.Split(src, types.AffinitySeparator)
	elementLen = min(len(dstElements), len(srcElements))

	// Maximum element length is 5.
	elementLen = min(elementLen, maxElementLen)

	for i := range elementLen {
		if !strings.EqualFold(dstElements[i], srcElements[i]) {
			break
		}

		score++
	}

	return float64(score) / float64(maxElementLen)
}

// calculateHostTypeScore 0.0~1.0 larger and better.
func (e *evaluatorDefault) calculateHostTypeScore(peer *standard.Peer) float64 {
	// When the task is downloaded for the first time,
	// peer will be scheduled to seed peer first,
	// otherwise it will be scheduled to dfdaemon first.
	if peer.Host.Type != types.HostTypeNormal {
		if peer.FSM.Is(standard.PeerStateReceivedNormal) ||
			peer.FSM.Is(standard.PeerStateRunning) {
			return maxScore
		}

		return minScore
	}

	return maxScore * 0.5
}
