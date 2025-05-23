// Copyright 2017 Fortio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"fortio.org/assert"
	"fortio.org/log"
)

const (
	ipOne = "127.0.0.1"
	ipTwo = "127.0.0.2"
)

func TestCounter(t *testing.T) {
	c := NewHistogram(22, 0.1)
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	c.Counter.Print(w, "test1c")
	expected := "test1c : count 0 avg 0 +/- 0 min 0 max 0 sum 0\n"
	c.Print(w, "test1h", []float64{50.0})
	expected += "test1h : no data\n"
	log.Config.LogFileAndLine = false
	log.Config.JSON = false
	log.SetFlags(0)
	log.SetOutput(w)
	c.Export().CalcPercentile(50)
	expected += "[E]> Unexpected call to CalcPercentile(50) with no data\n"
	c.Record(23.1)
	c.Counter.Print(w, "test2")
	expected += "test2 : count 1 avg 23.1 +/- 0 min 23.1 max 23.1 sum 23.1\n"
	c.Record(22.9)
	c.Counter.Print(w, "test3")
	expected += "test3 : count 2 avg 23 +/- 0.1 min 22.9 max 23.1 sum 46\n"
	c.Record(23.1)
	c.Record(22.9)
	c.Counter.Print(w, "test4")
	expected += "test4 : count 4 avg 23 +/- 0.1 min 22.9 max 23.1 sum 92\n"
	c.Record(1023)
	c.Record(-977)
	c.Counter.Print(w, "test5")
	// note that stddev of 577.4 below is... whatever the code said
	finalExpected := " : count 6 avg 23 +/- 577.4 min -977 max 1023 sum 138\n"
	expected += "test5" + finalExpected
	// Try the Log() function too:
	log.SetOutput(w)
	log.SetFlags(0)
	log.Config.LogFileAndLine = false
	log.Config.LogPrefix = ""
	c.Counter.Log("testLogC")
	expected += "[I] testLogC" + finalExpected
	_ = w.Flush()
	actual := b.String()
	if actual != expected {
		t.Errorf("unexpected1:\n%s\nvs expected:\n%s\n", actual, expected)
	}
	b.Reset()
	c.Log("testLogH", nil)
	_ = w.Flush()
	actual = b.String()
	expected = "[I] testLogH" + finalExpected + `# range, mid point, percentile, count
>= -977 <= 22 , -477.5 , 16.67, 1
> 22.8 <= 22.9 , 22.85 , 50.00, 2
> 23 <= 23.1 , 23.05 , 83.33, 2
> 1022 <= 1023 , 1022.5 , 100.00, 1
`
	if actual != expected {
		t.Errorf("unexpected2:\n%s\nvs expected:\n%s\n", actual, expected)
	}
	log.SetOutput(os.Stderr)
}

func TestTransferCounter(t *testing.T) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	var c1 Counter
	c1.Record(10)
	c1.Record(20)
	var c2 Counter
	c2.Record(80)
	c2.Record(90)
	c1a := c1
	c2a := c2
	var c3 Counter
	c1.Print(w, "c1 before merge")
	c2.Print(w, "c2 before merge")
	c1.Transfer(&c2)
	c1.Print(w, "mergedC1C2")
	c2.Print(w, "c2 after merge")
	// reverse (exercise min if)
	c2a.Transfer(&c1a)
	c2a.Print(w, "mergedC2C1")
	// test transfer into empty - min should be set
	c3.Transfer(&c1)
	c1.Print(w, "c1 should now be empty")
	c3.Print(w, "c3 after merge - 1")
	// test empty transfer - shouldn't reset min/no-op
	c3.Transfer(&c2)
	c3.Print(w, "c3 after merge - 2")
	_ = w.Flush()
	actual := b.String()
	expected := `c1 before merge : count 2 avg 15 +/- 5 min 10 max 20 sum 30
c2 before merge : count 2 avg 85 +/- 5 min 80 max 90 sum 170
mergedC1C2 : count 4 avg 50 +/- 35.36 min 10 max 90 sum 200
c2 after merge : count 0 avg 0 +/- 0 min 0 max 0 sum 0
mergedC2C1 : count 4 avg 50 +/- 35.36 min 10 max 90 sum 200
c1 should now be empty : count 0 avg 0 +/- 0 min 0 max 0 sum 0
c3 after merge - 1 : count 4 avg 50 +/- 35.36 min 10 max 90 sum 200
c3 after merge - 2 : count 4 avg 50 +/- 35.36 min 10 max 90 sum 200
`
	if actual != expected {
		t.Errorf("unexpected:\n%s\tvs:\n%s", actual, expected)
	}
}

func TestZeroDivider(t *testing.T) {
	h := NewHistogram(0, 0)
	if h != nil {
		t.Errorf("Histogram can not be created when divider is zero")
	}
}

func TestHistogram(t *testing.T) {
	h := NewHistogram(0, 10)
	h.Record(1)
	h.Record(251)
	h.Record(501)
	h.Record(751)
	h.Record(1001)
	h.Print(os.Stdout, "TestHistogram", []float64{50})
	e := h.Export()
	for i := 25; i <= 100; i += 25 {
		fmt.Printf("%d%% at %g\n", i, e.CalcPercentile(float64(i)))
	}
	tests := []struct {
		actual   float64
		expected float64
		msg      string
	}{
		{h.Avg(), 501, "avg"},
		{e.CalcPercentile(-1), 1, "p-1"}, // not valid but should return min
		{e.CalcPercentile(0), 1, "p0"},
		{e.CalcPercentile(0.1), 1, "p0.1"},
		{e.CalcPercentile(1), 1, "p1"},
		{e.CalcPercentile(20), 1, "p20"},             // 20% = first point, 1st bucket is 1-10
		{e.CalcPercentile(20.01), 250.025, "p20.01"}, // near beginning of bucket of 2nd pt
		{e.CalcPercentile(39.99), 299.975, "p39.99"},
		{e.CalcPercentile(40), 300, "p40"},
		{e.CalcPercentile(50), 550, "p50"},
		{e.CalcPercentile(75), 775, "p75"},
		{e.CalcPercentile(90), 1000.5, "p90"},
		{e.CalcPercentile(99), 1000.95, "p99"},
		{e.CalcPercentile(99.9), 1000.995, "p99.9"},
		{e.CalcPercentile(100), 1001, "p100"},
		{e.CalcPercentile(101), 1001, "p101"},
	}
	for _, tst := range tests {
		if tst.actual != tst.expected {
			t.Errorf("%s: got %g, not as expected %g", tst.msg, tst.actual, tst.expected)
		}
	}
}

func TestPercentiles1(t *testing.T) {
	h := NewHistogram(0, 10)
	h.Record(10)
	h.Record(20)
	h.Record(30)
	h.Print(os.Stdout, "TestPercentiles1", []float64{50})
	e := h.Export()
	for i := 0; i <= 100; i += 10 {
		fmt.Printf("%d%% at %g\n", i, e.CalcPercentile(float64(i)))
	}
	tests := []struct {
		actual   float64
		expected float64
		msg      string
	}{
		{h.Avg(), 20, "avg"},
		{e.CalcPercentile(-1), 10, "p-1"}, // not valid but should return min
		{e.CalcPercentile(0), 10, "p0"},
		{e.CalcPercentile(0.1), 10, "p0.1"},
		{e.CalcPercentile(1), 10, "p1"},
		{e.CalcPercentile(20), 10, "p20"},           // 20% = first point, 1st bucket is 10
		{e.CalcPercentile(33.33), 10, "p33.33"},     // near beginning of bucket of 2nd pt
		{e.CalcPercentile(100 / 3), 10, "p100/3"},   // near beginning of bucket of 2nd pt
		{e.CalcPercentile(33.34), 10.002, "p33.34"}, // near beginning of bucket of 2nd pt
		{e.CalcPercentile(50), 15, "p50"},
		{e.CalcPercentile(66.66), 19.998, "p66.66"},
		{e.CalcPercentile(100. * 2 / 3), 20, "p100*2/3"},
		{e.CalcPercentile(66.67), 20.001, "p66.67"},
		{e.CalcPercentile(75), 22.5, "p75"},
		{e.CalcPercentile(99), 29.7, "p99"},
		{e.CalcPercentile(99.9), 29.97, "p99.9"},
		{e.CalcPercentile(100), 30, "p100"},
		{e.CalcPercentile(101), 30, "p101"},
	}
	for _, tst := range tests {
		actualRounded := float64(int64(tst.actual*100000+0.5)) / 100000.
		if actualRounded != tst.expected {
			t.Errorf("%s: got %g (%g), not as expected %g", tst.msg, actualRounded, tst.actual, tst.expected)
		}
	}
}

func TestHistogramData(t *testing.T) {
	h := NewHistogram(0, 1)
	h.Record(-1)
	h.RecordN(0, 3)
	h.Record(1)
	h.Record(2)
	h.Record(3)
	h.Record(4)
	h.RecordN(5, 2)
	percs := []float64{0, 1, 10, 25, 40, 50, 60, 70, 80, 90, 99, 100}
	e := h.Export().CalcPercentiles(percs)
	e.Print(os.Stdout, "TestHistogramData")
	assert.CheckEquals(t, int64(10), e.Count, "10 data points")
	assert.CheckEquals(t, 1.9, e.Avg, "avg should be 2")
	assert.CheckEquals(t, e.Percentiles[0], Percentile{0, -1}, "p0 should be -1 (min)")
	assert.CheckEquals(t, e.Percentiles[1], Percentile{1, -1}, "p1 should be -1 (min)")
	assert.CheckEquals(t, e.Percentiles[2], Percentile{10, -1}, "p10 should be 1 (1/10 at min)")
	assert.CheckEquals(t, e.Percentiles[3], Percentile{25, -0.5}, "p25 should be half between -1 and 0")
	assert.CheckEquals(t, e.Percentiles[4], Percentile{40, 0}, "p40 should still be 0 (4/10 data pts at 0)")
	assert.CheckEquals(t, e.Percentiles[5], Percentile{50, 1}, "p50 should 1 (5th/10 point is 1)")
	assert.CheckEquals(t, e.Percentiles[6], Percentile{60, 2}, "p60 should 2 (6th/10 point is 2)")
	assert.CheckEquals(t, e.Percentiles[7], Percentile{70, 3}, "p70 should 3 (7th/10 point is 3)")
	assert.CheckEquals(t, e.Percentiles[8], Percentile{80, 4}, "p80 should 4 (8th/10 point is 4)")
	assert.CheckEquals(t, e.Percentiles[9], Percentile{90, 4.5}, "p90 should between 4 and 5 (2 points in bucket)")
	assert.CheckEquals(t, e.Percentiles[10], Percentile{99, 4.95}, "p99")
	assert.CheckEquals(t, e.Percentiles[11], Percentile{100, 5}, "p100 should 5 (10th/10 point is 5 and max is 5)")
	h.Log("test multi count", percs)
}

// Checks properties that should be true for all non-empty histograms.
func CheckGenericHistogramDataProperties(t *testing.T, e *HistogramData) {
	n := len(e.Data)
	if n <= 0 {
		t.Error("Unexpected empty histogram")

		return
	}
	assert.CheckEquals(t, e.Data[0].Start, e.Min, "first bucket starts at min")
	assert.CheckEquals(t, e.Data[n-1].End, e.Max, "end of last bucket is max")
	assert.CheckEquals(t, e.Data[n-1].Percent, 100., "last bucket is 100%")
	// All buckets in order
	var prev Bucket
	var sum int64
	for i := range n {
		b := e.Data[i]
		assert.Assert(t, b.Start <= b.End, "End %f should always be after Start %f", b.End, b.Start)
		assert.Assert(t, b.Count > 0, "Every exported bucket should have data")
		assert.Assert(t, b.Percent > 0, "Percentage should always be positive")
		sum += b.Count
		if i > 0 {
			assert.Assert(t, b.Start >= prev.End, "Start of next bucket >= end of previous")
			assert.Assert(t, b.Percent > prev.Percent, "Percentage should be ever increasing")
		}
		prev = b
	}
	assert.CheckEquals(t, sum, e.Count, "Sum in buckets should add up to Counter's count")
}

func TestHistogramExport1(t *testing.T) {
	h := NewHistogram(0, 10)
	e := h.Export()
	assert.CheckEquals(t, e.Count, int64(0), "empty is 0 count")
	assert.CheckEquals(t, len(e.Data), 0, "empty is no bucket data")
	h.Record(-137.4)
	h.Record(251)
	h.Record(501)
	h.Record(751)
	h.Record(1001.67)
	e = h.Export().CalcPercentiles([]float64{50, 99, 99.9})
	assert.CheckEquals(t, e.Count, int64(5), "count")
	assert.CheckEquals(t, e.Min, -137.4, "min")
	assert.CheckEquals(t, e.Max, 1001.67, "max")
	n := len(e.Data)
	assert.CheckEquals(t, n, 5, "number of buckets")
	CheckGenericHistogramDataProperties(t, e)
	data, err := json.MarshalIndent(e, "", " ")
	if err != nil {
		t.Error(err)
	}
	assert.CheckEquals(t, string(data), `{
 "Count": 5,
 "Min": -137.4,
 "Max": 1001.67,
 "Sum": 2367.27,
 "Avg": 473.454,
 "StdDev": 394.8242896074151,
 "Data": [
  {
   "Start": -137.4,
   "End": 0,
   "Percent": 20,
   "Count": 1
  },
  {
   "Start": 250,
   "End": 300,
   "Percent": 40,
   "Count": 1
  },
  {
   "Start": 500,
   "End": 600,
   "Percent": 60,
   "Count": 1
  },
  {
   "Start": 700,
   "End": 800,
   "Percent": 80,
   "Count": 1
  },
  {
   "Start": 1000,
   "End": 1001.67,
   "Percent": 100,
   "Count": 1
  }
 ],
 "Percentiles": [
  {
   "Percentile": 50,
   "Value": 550
  },
  {
   "Percentile": 99,
   "Value": 1001.5865
  },
  {
   "Percentile": 99.9,
   "Value": 1001.66165
  }
 ]
}`, "Json output")
}

const (
	NumRandomHistogram = 10000
)

func TestHistogramNegativeOffset(t *testing.T) {
	// Check case where offset is negative and min value also negative
	h := NewHistogram(-453.726, 94.80)
	h.Record(-453.721)
	h.Record(252)
	CheckGenericHistogramDataProperties(t, h.Export())
}

func TestHistogramExportRandom(t *testing.T) {
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	for i := range NumRandomHistogram {
		// offset [-500,500[  divisor ]0,100]
		offset := (r.Float64() - 0.5) * 1000
		div := 100 * (1 - r.Float64())
		numEntries := 1 + r.Int31n(10000)
		// fmt.Printf("new histogram with offset %g, div %g - will insert %d entries\n", offset, div, numEntries)
		h := NewHistogram(offset, div)
		var n int32
		var minV float64
		var maxV float64
		for ; n < numEntries; n++ {
			v := 3000 * (r.Float64() - 0.25)
			if n == 0 {
				minV = v
				maxV = v
			} else {
				if v < minV {
					minV = v
				} else if v > maxV {
					maxV = v
				}
			}
			h.Record(v)
		}
		e := h.Export().CalcPercentiles([]float64{0, 50, 100})
		CheckGenericHistogramDataProperties(t, e)
		assert.CheckEquals(t, h.Count, int64(numEntries), "num entries should match")
		assert.CheckEquals(t, h.Min, minV, "Min should match")
		assert.CheckEquals(t, h.Max, maxV, "Max should match")
		assert.CheckEquals(t, e.Percentiles[0].Value, minV, "p0 should be min")
		assert.CheckEquals(t, e.Percentiles[2].Value, maxV, "p100 should be max")
		if t.Failed() {
			t.Logf("Failed seed %v iter %d, offset %v, div %v, numEntries %v", seed, i, offset, div, numEntries)
			t.Logf("%v", e)
			t.FailNow()
		}
	}
}

func TestHistogramLastBucket(t *testing.T) {
	// Use -1 offset, so first bucket is negative values
	h := NewHistogram( /* offset */ -1 /*scale */, 1)
	h.Record(-1)
	h.Record(0)
	h.Record(1)
	h.Record(3)
	h.Record(10)
	h.Record(99999)  // last value of one before last bucket 100k-offset
	h.Record(100000) // first value of the extra bucket
	h.Record(200000)
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	h.Print(w, "testLastBucket", []float64{90})
	_ = w.Flush()
	actual := b.String()
	// stdev part is not verified/could be brittle
	expected := `testLastBucket : count 8 avg 50001.5 +/- 7.071e+04 min -1 max 200000 sum 400012
# range, mid point, percentile, count
>= -1 <= -1 , -1 , 12.50, 1
> -1 <= 0 , -0.5 , 25.00, 1
> 0 <= 1 , 0.5 , 37.50, 1
> 2 <= 3 , 2.5 , 50.00, 1
> 9 <= 10 , 9.5 , 62.50, 1
> 74999 <= 99999 , 87499 , 75.00, 1
> 99999 <= 200000 , 150000 , 100.00, 2
# target 90% 160000
`
	if actual != expected {
		t.Errorf("unexpected:\n%s\tvs:\n%s", actual, expected)
	}
}

func TestHistogramNegativeNumbers(t *testing.T) {
	h := NewHistogram( /* offset */ -10 /*scale */, 1)
	h.Record(-10)
	h.Record(10)
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	h.Print(w, "testHistogramWithNegativeNumbers", []float64{1, 50, 75})
	_ = w.Flush()
	actual := b.String()
	// stdev part is not verified/could be brittle
	expected := `testHistogramWithNegativeNumbers : count 2 avg 0 +/- 10 min -10 max 10 sum 0
# range, mid point, percentile, count
>= -10 <= -10 , -10 , 50.00, 1
> 8 <= 10 , 9 , 100.00, 1
# target 1% -10
# target 50% -10
# target 75% 9
`
	if actual != expected {
		t.Errorf("unexpected:\n%s\tvs:\n%s", actual, expected)
	}
}

func TestMergeHistogramsWithDifferentScales(t *testing.T) {
	tP := []float64{100.}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	h1 := NewHistogram(0, 10)
	h1.Record(20)
	h1.Record(50)
	h1.Record(90)
	h2 := NewHistogram(2, 100)
	h2.Record(30)
	h2.Record(40)
	h2.Record(50)
	newH := Merge(h1, h2)
	newH.Print(w, "h1 and h2 merged", tP)
	w.Flush()
	actual := b.String()
	expected := `h1 and h2 merged : count 6 avg 46.666667 +/- 22.11 min 20 max 90 sum 280
# range, mid point, percentile, count
>= 20 <= 90 , 55 , 100.00, 6
# target 100% 90
`
	if newH.Divider != h2.Divider {
		t.Errorf("unexpected:\n%f\tvs:\n%f", newH.Divider, h2.Divider)
	}
	if newH.Offset != h1.Offset {
		t.Errorf("unexpected:\n%f\tvs:\n%f", newH.Offset, h1.Offset)
	}
	if actual != expected {
		t.Errorf("unexpected:\n%s\tvs:\n%s", actual, expected)
	}

	b.Reset()
	h3 := NewHistogram(5, 200)
	h3.Record(10000)
	h3.Record(5000)
	h3.Record(9000)
	h4 := NewHistogram(2, 100)
	h4.Record(300)
	h4.Record(400)
	h4.Record(50)
	newH = Merge(h3, h4)
	newH.Print(w, "h3 and h4 merged", tP)
	w.Flush()
	actual = b.String()
	expected = `h3 and h4 merged : count 6 avg 4125 +/- 4167 min 50 max 10000 sum 24750
# range, mid point, percentile, count
>= 50 <= 202 , 126 , 16.67, 1
> 202 <= 402 , 302 , 50.00, 2
> 5002 <= 6002 , 5502 , 66.67, 1
> 8002 <= 9002 , 8502 , 83.33, 1
> 9002 <= 10000 , 9501 , 100.00, 1
# target 100% 10000
`
	if newH.Divider != h3.Divider {
		t.Errorf("unexpected:\n%f\tvs:\n%f", newH.Divider, h3.Divider)
	}
	if newH.Offset != h4.Offset {
		t.Errorf("unexpected:\n%f\tvs:\n%f", newH.Offset, h4.Offset)
	}
	if actual != expected {
		t.Errorf("unexpected:\n%s\tvs:\n%s", actual, expected)
	}
}

func TestTransferHistogramWithDifferentScales(t *testing.T) {
	tP := []float64{75.}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	h1 := NewHistogram(2, 15)
	h1.Record(30)
	h1.Record(40)
	h1.Record(50)
	h2 := NewHistogram(0, 10)
	h2.Record(20)
	h2.Record(23)
	h2.Record(90)
	h1.Print(w, "h1 before merge", tP)
	h2.Print(w, "h2 before merge", tP)
	h1.Transfer(h2)
	h1.Print(w, "merged h2 -> h1", tP)
	h2.Print(w, "h2 should now be empty", tP)
	w.Flush()
	actual := b.String()
	expected := `h1 before merge : count 3 avg 40 +/- 8.165 min 30 max 50 sum 120
# range, mid point, percentile, count
>= 30 <= 32 , 31 , 33.33, 1
> 32 <= 47 , 39.5 , 66.67, 1
> 47 <= 50 , 48.5 , 100.00, 1
# target 75% 47.75
h2 before merge : count 3 avg 44.333333 +/- 32.31 min 20 max 90 sum 133
# range, mid point, percentile, count
>= 20 <= 20 , 20 , 33.33, 1
> 20 <= 30 , 25 , 66.67, 1
> 80 <= 90 , 85 , 100.00, 1
# target 75% 82.5
merged h2 -> h1 : count 6 avg 42.166667 +/- 23.67 min 20 max 90 sum 253
# range, mid point, percentile, count
>= 20 <= 32 , 26 , 50.00, 3
> 32 <= 47 , 39.5 , 66.67, 1
> 47 <= 62 , 54.5 , 83.33, 1
> 77 <= 90 , 83.5 , 100.00, 1
# target 75% 54.5
h2 should now be empty : no data
`
	if actual != expected {
		t.Errorf("unexpected:\n%s\tvs:\n%s", actual, expected)
	}
}

func TestTransferHistogram(t *testing.T) {
	tP := []float64{75}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	h1 := NewHistogram(0, 10)
	h1.Record(10)
	h1.Record(20)
	h2 := NewHistogram(0, 10)
	h2.Record(80)
	h2.Record(90)
	h1a := h1.Clone()
	h1a.Record(50) // add extra pt to make sure h1a and h1 are distinct
	h2a := h2.Clone()
	h3 := NewHistogram(0, 10)
	h1.Print(w, "h1 before merge", tP)
	h2.Print(w, "h2 before merge", tP)
	h1.Transfer(h2)
	h1.Print(w, "merged h2 -> h1", tP)
	h2.Print(w, "h2 after merge", tP)
	// reverse (exercise min if)
	h2a.Transfer(h1a)
	h2a.Print(w, "merged h1a -> h2a", tP)
	// test transfer into empty - min should be set
	h3.Transfer(h1)
	h1.Print(w, "h1 should now be empty", tP)
	h3.Print(w, "h3 after merge - 1", tP)
	// test empty transfer - shouldn't reset min/no-op
	h3.Transfer(h2)
	h3.Print(w, "h3 after merge - 2", tP)
	_ = w.Flush()
	actual := b.String()
	expected := `h1 before merge : count 2 avg 15 +/- 5 min 10 max 20 sum 30
# range, mid point, percentile, count
>= 10 <= 10 , 10 , 50.00, 1
> 10 <= 20 , 15 , 100.00, 1
# target 75% 15
h2 before merge : count 2 avg 85 +/- 5 min 80 max 90 sum 170
# range, mid point, percentile, count
>= 80 <= 80 , 80 , 50.00, 1
> 80 <= 90 , 85 , 100.00, 1
# target 75% 85
merged h2 -> h1 : count 4 avg 50 +/- 35.36 min 10 max 90 sum 200
# range, mid point, percentile, count
>= 10 <= 10 , 10 , 25.00, 1
> 10 <= 20 , 15 , 50.00, 1
> 70 <= 80 , 75 , 75.00, 1
> 80 <= 90 , 85 , 100.00, 1
# target 75% 80
h2 after merge : no data
merged h1a -> h2a : count 5 avg 50 +/- 31.62 min 10 max 90 sum 250
# range, mid point, percentile, count
>= 10 <= 10 , 10 , 20.00, 1
> 10 <= 20 , 15 , 40.00, 1
> 40 <= 50 , 45 , 60.00, 1
> 70 <= 80 , 75 , 80.00, 1
> 80 <= 90 , 85 , 100.00, 1
# target 75% 77.5
h1 should now be empty : no data
h3 after merge - 1 : count 4 avg 50 +/- 35.36 min 10 max 90 sum 200
# range, mid point, percentile, count
>= 10 <= 10 , 10 , 25.00, 1
> 10 <= 20 , 15 , 50.00, 1
> 70 <= 80 , 75 , 75.00, 1
> 80 <= 90 , 85 , 100.00, 1
# target 75% 80
h3 after merge - 2 : count 4 avg 50 +/- 35.36 min 10 max 90 sum 200
# range, mid point, percentile, count
>= 10 <= 10 , 10 , 25.00, 1
> 10 <= 20 , 15 , 50.00, 1
> 70 <= 80 , 75 , 75.00, 1
> 80 <= 90 , 85 , 100.00, 1
# target 75% 80
`
	if actual != expected {
		t.Errorf("unexpected:\n%s\tvs:\n%s", actual, expected)
	}
}

func TestParsePercentiles(t *testing.T) {
	tests := []struct {
		str  string    // input
		list []float64 // expected
		err  bool
	}{
		// Good cases
		{str: "99.9", list: []float64{99.9}},
		{str: "1,2,3", list: []float64{1, 2, 3}},
		{str: "   17, 5.3,  78  ", list: []float64{17, 5.3, 78}},
		// Errors
		{str: "1024", list: []float64{}, err: true},
		{str: "0", list: []float64{}, err: true},
		{str: "100", list: []float64{}, err: true},
		{str: "-1", list: []float64{}, err: true},
		{str: "", list: []float64{}, err: true},
		{str: "   ", list: []float64{}, err: true},
		{str: "23,a,46", list: []float64{23}, err: true},
	}
	log.SetLogLevel(log.Debug) // for coverage
	for _, tst := range tests {
		actual, err := ParsePercentiles(tst.str)
		if !reflect.DeepEqual(actual, tst.list) {
			t.Errorf("ParsePercentiles got %#v expected %#v", actual, tst.list)
		}
		if (err != nil) != tst.err {
			t.Errorf("ParsePercentiles got %v error while expecting err:%v for %s",
				err, tst.err, tst.str)
		}
	}
}

func TestRound(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{100.00001, 100},
		{100.0001, 100.0001},
		{99.9999999999, 100},
		{100, 100},
		{0.1234499, 0.1234},
		{0.1234567, 0.1235},
		{-0.0000049, 0},
		{-0.499999, -0.5},
		{-0.500001, -0.5},
		{-0.999999, -1},
		{-0.123449, -0.1234},
		{-0.123450, -0.1234}, // should be -0.1235 but we don't deal with <0 specially
	}
	for _, tst := range tests {
		if actual := Round(tst.input); actual != tst.expected {
			t.Errorf("Got %f, expected %f for Round(%.10f)", actual, tst.expected, tst.input)
		}
	}
}

func TestNaN(t *testing.T) {
	var c Counter
	for i := 599713; i > 0; i-- {
		c.Record(1281)
	}
	c.Log("counter with 599713 times 1281 - issue #97")
	actual := c.StdDev()
	if actual != 0.0 {
		t.Errorf("Got %g, expected 0 for stddev/issue #97 c is %+v", actual, c)
	}
}

func TestBucketLookUp(t *testing.T) {
	tests := []struct {
		input float64 // input
		start float64 // start
		end   float64 // end
	}{
		{input: 11, start: 10, end: 11},
		{input: 171, start: 160, end: 180},
		{input: 180, start: 160, end: 180},
		{input: 801, start: 800, end: 900},
		{input: 900, start: 800, end: 900},
		{input: 999, start: 900, end: 1000},
		{input: 999.99, start: 900, end: 1000},
		{input: 1000, start: 900, end: 1000},
		{input: 1000.01, start: 1000, end: 2000},
		{input: 1001, start: 1000, end: 2000},
		{input: 1001.1, start: 1000, end: 2000},
		{input: 1999.99, start: 1000, end: 2000},
		{input: 2000, start: 1000, end: 2000},
		{input: 2001, start: 2000, end: 3000},
		{input: 2001.1, start: 2000, end: 3000},
		{input: 75000, start: 50000, end: 75000},
		{input: 75000.01, start: 75000, end: 100000},
		{input: 99999.99, start: 75000, end: 100000},
		{input: 100000, start: 75000, end: 100000},
		{input: 100000.01, start: 100000, end: 1e+06},
		{input: 100000.99, start: 100000, end: 1e+06},
		{input: 100001, start: 100000, end: 1e+06},
		{input: 523477, start: 100000, end: 1e+06},
	}
	h := NewHistogram(0, 1)
	for _, test := range tests {
		h.Reset()
		h.Record(1)
		h.Record(1000000)
		h.Record(test.input)
		hData := h.Export()
		hd := hData.Data[1]
		if hd.Start != test.start || hd.End != test.end {
			t.Errorf("Got %+v while expected %+v", hd, test)
		}
	}
}

func TestAllBucketBoundaries(t *testing.T) {
	h := NewHistogram(0, 1)

	for i, value := range histogramBucketValues {
		v := float64(value)
		h.Reset()
		h.Record(-1)
		h.RecordN(v-0.01, 50)
		h.RecordN(v, 700)       // so first interval gets a count of 750 + 1 for first
		h.RecordN(v+0.01, 1003) // second interval gets a count of 1003 + 1 for last
		h.Record(1e6)
		hData := h.Export()
		var firstInterval int64
		if i == 0 {
			firstInterval = 1 // fist interval is [min, 0[
		}
		var lastInterval int64
		if i == len(histogramBucketValues)-1 {
			lastInterval = 1
		}
		if hData.Data[1-firstInterval].End != v || hData.Data[1-firstInterval].Count != 750+firstInterval {
			t.Errorf("= Boundary, got %+v unexpectedly for %d %g", hData, value, v)
		}
		if hData.Data[2-firstInterval].Start != v || hData.Data[2-firstInterval].Count != 1003+lastInterval {
			t.Errorf("> Boundary, got %+v unexpectedly for %d %g", hData, value, v)
		}
	}
}

// Unit tests for Occurrence.
func TestSingleIPOccurrence(t *testing.T) {
	singleIP := NewOccurrence()
	totalMap := make(map[string]int)
	expected := "127.0.0.1"

	singleIP.Record(ipOne)
	singleIP.Record(ipOne)

	actual := singleIP.AggregateAndToString(totalMap)

	if expected != actual {
		t.Errorf("Incorrect IP Usage Result. Expected: %s, got: %s", expected, actual)
	}
}

func TestMultipleIPOccurrence(t *testing.T) {
	multiIP := NewOccurrence()
	totalMap := make(map[string]int)
	expected := [2]string{"[127.0.0.1 (2), 127.0.0.2 (1)]", "[127.0.0.2 (1), 127.0.0.1 (2)]"}

	multiIP.Record(ipOne)
	multiIP.Record(ipOne)

	multiIP.Record(ipTwo)

	actual := multiIP.AggregateAndToString(totalMap)

	if actual != expected[0] && actual != expected[1] {
		t.Errorf("Incorrect IP Usage Result. Expected: %s, got: %s", expected, actual)
	}
}

func TestMultipleConnIPOccurrence(t *testing.T) {
	occurrenceOne := NewOccurrence()
	occurrenceTwo := NewOccurrence()
	totalMap := make(map[string]int)

	// IP usage for first connection.
	occurrenceOne.Record(ipOne)
	occurrenceOne.Record(ipOne)
	occurrenceOne.Record(ipOne)
	occurrenceOne.Record(ipTwo)

	// IP usage for second connection.
	occurrenceTwo.Record(ipOne)
	occurrenceTwo.Record(ipOne)
	occurrenceTwo.Record(ipTwo)
	occurrenceTwo.Record(ipTwo)

	_ = occurrenceOne.AggregateAndToString(totalMap)
	_ = occurrenceTwo.AggregateAndToString(totalMap)

	// The occurrence of IP one should be 5 and the occurrence of IP two should be 3.
	if totalMap[ipOne] != 5 || totalMap[ipTwo] != 3 {
		t.Errorf("Incorrect total IP usage count.")
	}
}

// TODO: add test with data 1.0 1.0001 1.999 2.0 2.5
// should get 3 buckets 0-1 with count 1
// 1-2 with count 3
// 2-2.5 with count 1

func BenchmarkBucketLookUpWithHighestValue(b *testing.B) {
	testHistogram := NewHistogram(0, 1)
	for i := 0; i < b.N; i++ {
		testHistogram.Record(99999)
	}
}

func BenchmarkBucketLookUpWithRandomValues(b *testing.B) {
	testHistogram := NewHistogram(0, 1)
	for i := 0; i < b.N; i++ {
		testHistogram.Record(float64(rand.Intn(100000)))
	}
}
