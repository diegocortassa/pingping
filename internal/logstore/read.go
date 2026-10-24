package logstore

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/diegocortassa/pingping/internal/model"
)

// requiredColumns are the columns a log file must have to be readable.
var requiredColumns = []string{"timestamp", "address", "rtt_ms", "success"}

// Read parses a log file written by a Writer. The rows that cannot be parsed
// are skipped, an empty result means the file holds no usable sample.
func Read(path string) ([]model.Sample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open log %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read log header %s: %w", path, err)
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		columns[strings.TrimSpace(name)] = i
	}
	for _, name := range requiredColumns {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("log %s has no %q column", path, name)
		}
	}

	var samples []model.Sample
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if errors.Is(err, csv.ErrFieldCount) {
				continue
			}
			return nil, fmt.Errorf("read log %s: %w", path, err)
		}
		sample, ok := sampleOf(record, columns)
		if ok {
			samples = append(samples, sample)
		}
	}

	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Time.Before(samples[j].Time)
	})
	return samples, nil
}

// sampleOf converts a record into a sample. The second result is false when
// the record cannot be parsed.
func sampleOf(record []string, columns map[string]int) (model.Sample, bool) {
	var sample model.Sample

	stamp := column(record, columns, "timestamp")
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return sample, false
	}
	sample.Time = parsed

	millis, err := strconv.ParseFloat(column(record, columns, "rtt_ms"), 64)
	if err != nil {
		return sample, false
	}
	sample.RTT = time.Duration(millis * float64(time.Millisecond))

	success, err := strconv.ParseBool(column(record, columns, "success"))
	if err != nil {
		return sample, false
	}
	sample.Success = success

	sample.TargetID = column(record, columns, "target_id")
	sample.Address = column(record, columns, "address")
	sample.Error = column(record, columns, "error")
	return sample, true
}

// column returns a field of a record, or an empty string when the column is
// missing.
func column(record []string, columns map[string]int, name string) string {
	index, ok := columns[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}
