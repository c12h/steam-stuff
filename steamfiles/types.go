package steamfiles // import "github.com/c12h/steam-stuff/steamfiles"

import (
	"fmt"
	"math"
	"strconv"
)

// Steam identifies apps by a positive integer.
//
// As of January 2021, the largest app ID in use is 2,028,850, so int32 is wide
// enough.
//
type AppNum int32

// parseAppNum gets an AppNum from a string, with lots of error checking.
//
func parseAppNum(text, path string) (AppNum, error) {
	appNum, err := strconv.Atoi(text)
	if err != nil {
		return 0, fileError(path, "", "has app ID %q, need integer", text)
	}
	if appNum > math.MaxInt32 {
		panic(fmt.Sprintf("app ID %d from file %q is too big for int32!",
			appNum, path))
	} else if appNum <= 0 {
		return 0, fileError(path, "", "has app ID %d!?", appNum)
	}
	return AppNum(appNum), nil
}
