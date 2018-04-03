package purger

import (
	"os"

	"github.com/go-kit/kit/log"
)

var logger = log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), "caller", log.Caller(5))
