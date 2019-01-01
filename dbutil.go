package migration

import "time"

// timeVal implements the sql.Scanner method, and is a forgiving
// scanner for time values. This is useful when working with sqlite,
// which stores time values as text or int64. (It also supports
// float64, but this little type doesn't).
type timeVal struct {
	Time time.Time
}

func (tv *timeVal) Scan(src interface{}) error {
	if src == nil {
		tv.Time = time.Unix(0, 0).UTC()
		return nil
	}

	switch v := src.(type) {
	case time.Time:
		tv.Time = v
		return nil
	case string:
		for _, format := range []string{
			"2006-01-02 15:04:05Z07:00", // sqlite
			time.RFC3339,
			time.RFC3339Nano,
		} {
			if tm, err := time.Parse(format, v); err == nil {
				tv.Time = tm
				return nil
			}
		}
	case int64:
		tv.Time = time.Unix(v, 0).UTC()
		return nil
	}

	tv.Time = time.Unix(0, 0).UTC()
	return nil
}
