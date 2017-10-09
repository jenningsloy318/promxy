package promhttputil

import (
	"fmt"
	"reflect"

	"github.com/prometheus/common/model"
)

// TODO: always make copies? Now we sometimes return one, or make a copy, or do nothing
// Merge 2 values and
func MergeValues(a, b model.Value) (model.Value, error) {
	if a.Type() != b.Type() {
		return nil, fmt.Errorf("Error!")
	}

	switch aTyped := a.(type) {
	// TODO: more logic? for now we assume both are correct if they exist
	// In the case where it is a single datapoint, we're going to assume that
	// either is valid, we just need one
	case *model.Scalar:
		bTyped := b.(*model.Scalar)

		if aTyped.Value != 0 && aTyped.Timestamp != 0 {
			return aTyped, nil
		} else {
			return bTyped, nil
		}

	// In the case where it is a single datapoint, we're going to assume that
	// either is valid, we just need one
	case *model.String:
		bTyped := b.(*model.String)

		if aTyped.Value != "" && aTyped.Timestamp != 0 {
			return aTyped, nil
		} else {
			return bTyped, nil
		}

	// List of *model.Sample -- only 1 value (guaranteed same timestamp)
	case model.Vector:
		bTyped := b.(model.Vector)

		newValue := make(model.Vector, 0, len(aTyped)+len(bTyped))
		fingerPrintMap := make(map[model.Fingerprint]int)

		addItem := func(item *model.Sample) {
			finger := item.Metric.Fingerprint()

			// If we've seen this fingerPrint before, lets make sure that a value exists
			if index, ok := fingerPrintMap[finger]; ok {
				// TODO: better? For now we only replace if we have no value (which seems reasonable)
				if newValue[index].Value == model.SampleValue(0) {
					newValue[index].Value = item.Value
				}
			} else {
				newValue = append(newValue, item)
				fingerPrintMap[finger] = len(newValue) - 1
			}
		}

		for _, item := range aTyped {
			addItem(item)
		}

		for _, item := range bTyped {
			addItem(item)
		}
		return newValue, nil

	case model.Matrix:
		bTyped := b.(model.Matrix)

		newValue := make(model.Matrix, 0, len(aTyped)+len(bTyped))
		fingerPrintMap := make(map[model.Fingerprint]int)

		addStream := func(stream *model.SampleStream) {
			finger := stream.Metric.Fingerprint()

			// If we've seen this fingerPrint before, lets make sure that a value exists
			if index, ok := fingerPrintMap[finger]; ok {
				// TODO: check this error? For now the only one is sig collision, which we check
				newValue[index], _ = MergeSampleStream(newValue[index], stream)
			} else {
				newValue = append(newValue, stream)
				fingerPrintMap[finger] = len(newValue) - 1
			}
		}

		for _, item := range aTyped {
			addStream(item)
		}

		for _, item := range bTyped {
			addStream(item)
		}
		return newValue, nil
	}

	return nil, fmt.Errorf("Unknown type! %v", reflect.TypeOf(a))
}

func MergeSampleStream(a, b *model.SampleStream) (*model.SampleStream, error) {
	if a.Metric.Fingerprint() != b.Metric.Fingerprint() {
		return nil, fmt.Errorf("Cannot merge mismatch fingerprints")
	}

	// TODO: really there should be a library method for this in prometheus IMO
	// At this point we have 2 sorted lists of datapoints which we need to merge
	seenTimes := make(map[model.Time]struct{})
	newValues := make([]model.SamplePair, 0, len(a.Values)+len(b.Values))

	ai := 0 // Offset in a
	bi := 0 // Offset in b

	for {
		if ai >= len(a.Values) && bi >= len(b.Values) {
			break
		}

		var item model.SamplePair

		if ai < len(a.Values) { // If a exists
			if bi < len(b.Values) {
				// both items
				if a.Values[ai].Timestamp < b.Values[bi].Timestamp {
					item = a.Values[ai]
					ai++
				} else {
					item = b.Values[bi]
					bi++
				}
			} else {
				// Only A
				item = a.Values[ai]
				ai++
			}
		} else {
			if bi < len(b.Values) {
				// Only B
				item = b.Values[bi]
				bi++
			}
		}
		// If we've already seen this timestamp, skip
		if _, ok := seenTimes[item.Timestamp]; ok {
			continue
		}

		// Otherwise, lets add it
		newValues = append(newValues, item)
		seenTimes[item.Timestamp] = struct{}{}
	}

	return &model.SampleStream{
		Metric: a.Metric,
		Values: newValues,
	}, nil
}