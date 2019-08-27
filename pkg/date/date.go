package date

import (
	"reflect"
	"time"

	"github.com/pinpt/go-common/datetime"
)

// ConvertToModel will fill dateModel based on passed time
func ConvertToModel(ts time.Time, dateModel interface{}) {
	if ts.IsZero() {
		return
	}

	date, err := datetime.NewDateWithTime(ts)
	if err != nil {
		// this will never happen NewDateWithTime, always returns nil
		panic(err)
	}

	t := reflect.ValueOf(dateModel).Elem()
	t.FieldByName("Rfc3339").Set(reflect.ValueOf(date.Rfc3339))
	t.FieldByName("Epoch").Set(reflect.ValueOf(date.Epoch))
	t.FieldByName("Offset").Set(reflect.ValueOf(date.Offset))
}
