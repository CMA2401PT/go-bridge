package mapsample

func Echo(value map[string]int) map[string]int       { return value }
func Put(value map[string]int, key string, item int) { value[key] = item }
func Nil() map[string]int                            { return nil }
