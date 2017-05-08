// Copyright 2016 David Lazar. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bloom

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"log"
	"math"
	"testing"
)

func TestBitset(t *testing.T) {
	f := New(1024, 4)
	for i := uint32(0); i < 1024; i++ {
		if f.test(i) {
			t.Fatalf("bit %d should not be set: %#v", i, f.data)
		}
		f.set(i)
		if !f.test(i) {
			t.Fatalf("bit %d should be set", i)
		}
	}
}

func TestFilter(t *testing.T) {
	f := New(1024, 4)
	if f.Test([]byte("foo")) {
		t.Fatalf("foo not expected")
	}
	f.Set([]byte("foo"))
	if !f.Test([]byte("foo")) {
		t.Fatalf("foo expected")
	}
}

func TestOptimal(t *testing.T) {
	numElementsCases := []int{100, 10000, 100000}
	fpRateCases := []float64{0.001, 0.00001, 0.0000001}
	// increasing numFP can reduce error, but makes the tests take longer
	numFP := []int{100, 25, 5}

	if testing.Short() {
		numElementsCases = []int{100, 100000}
		fpRateCases = []float64{0.001, 0.00001}
		numFP = []int{100, 25}
	}

	for _, numElements := range numElementsCases {
		for i, fpRate := range fpRateCases {
			f := New(Optimal(numElements, fpRate))
			actualRate := f.estimateFalsePositiveRate(uint32(numElements), numFP[i])
			if actualRate < fpRate {
				if testing.Verbose() {
					log.Printf("\tok: numElements=%v want %v, got %v", numElements, fpRate, actualRate)
				}
				continue
			}
			ok, err := closeEnough(fpRate, actualRate, 0.20)
			if ok {
				if testing.Verbose() {
					log.Printf("\tok: numElements=%v want %v, got %v (%.2f%% error)", numElements, fpRate, actualRate, err*100)
				}
				continue
			}

			t.Fatalf("numElements=%v want %v, got %v (%.2f%% error)", numElements, fpRate, actualRate, err*100)
		}
	}
}

func closeEnough(a, b, maxerr float64) (bool, float64) {
	var relerr float64
	if math.Abs(b) > math.Abs(a) {
		relerr = math.Abs((a - b) / b)
	} else {
		relerr = math.Abs((a - b) / a)
	}
	if relerr <= maxerr {
		return true, relerr
	}
	return false, relerr
}

// based on "github.com/willf/bloom"
func (f *Filter) estimateFalsePositiveRate(numAdded uint32, numFP int) float64 {
	x := make([]byte, 4)
	for i := uint32(0); i < numAdded; i++ {
		binary.BigEndian.PutUint32(x, i)
		f.Set(x)
	}

	falsePositives := 0
	numRounds := 0
	for i := uint32(0); falsePositives < numFP; i++ {
		binary.BigEndian.PutUint32(x, numAdded+i+1)
		if f.Test(x) {
			falsePositives++
		}
		numRounds++
	}

	return float64(falsePositives) / float64(numRounds)
}

func TestOptimalSize(t *testing.T) {
	// These are the parameters we use in the Alpenhorn paper.
	numElements := 150000
	f := New(Optimal(numElements, 1e-10))
	bs, _ := f.MarshalBinary()
	bitsPerElement := math.Ceil(float64(len(bs)) * 8.0 / float64(numElements))
	if bitsPerElement != 48 {
		t.Fatalf("got %v bits per element, want %v", bitsPerElement, 48)
	}
}

func TestIncompressible(t *testing.T) {
	numElements := 150000
	filter := New(Optimal(numElements, 1e-10))
	x := make([]byte, 4)
	for i := uint32(0); i < uint32(numElements); i++ {
		binary.BigEndian.PutUint32(x, i)
		filter.Set(x)
	}
	filterBytes, _ := filter.MarshalBinary()

	compressed := new(bytes.Buffer)
	w, _ := flate.NewWriter(compressed, 9)
	w.Write(filterBytes)
	w.Close()
	if compressed.Len() < len(filterBytes)*99/100 {
		t.Fatalf("Compressed %d -> %d", len(filterBytes), compressed.Len())
	}
}

func BenchmarkCreateLargeFilter(b *testing.B) {
	// dialing mu=25000; 3 servers; so each mailbox is 75000 real and 75000 noise
	// for a total of 150000 elements in the dialing bloom filter
	numElements := 150000
	for i := 0; i < b.N; i++ {
		f := New(Optimal(numElements, 1e-10))
		x := make([]byte, 4)
		for i := uint32(0); i < uint32(numElements); i++ {
			binary.BigEndian.PutUint32(x, i)
			f.Set(x)
		}
	}
}
