package date

import (
	"reflect"
	"time"
)

// ConvertToModel will fill dateModel based on passed time
func ConvertToModel(ts time.Time, dateModel interface{}) {
	t := reflect.ValueOf(dateModel).Elem()
	t.FieldByName("Rfc3339").Set(reflect.ValueOf(ts.Format(time.RFC3339)))
	t.FieldByName("Epoch").Set(reflect.ValueOf(ts.Unix()))
	_, offset := ts.Zone()
	t.FieldByName("Offset").Set(reflect.ValueOf(int64(offset)))
}
