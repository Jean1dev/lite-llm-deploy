//go:build race

package jsonutil

func init() {
	raceEnabled = true
}
