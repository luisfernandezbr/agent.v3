package date

import (
	"reflect"
	"time"

	"github.com/pinpt/go-common/v10/datetime"
)

// ConvertToModel will fill dateModel based on passed time
func ConvertToModel(ts time.Time, dateModel interface{}) {
	if ts.IsZero() {
		return
	}

	date := datetime.NewDateWithTime(ts)
	t := reflect.ValueOf(dateModel).Elem()
	t.FieldByName("Rfc3339").Set(reflect.ValueOf(date.Rfc3339))
	t.FieldByName("Epoch").Set(reflect.ValueOf(date.Epoch))
	t.FieldByName("Offset").Set(reflect.ValueOf(date.Offset))
}
