package xlogadapter

import (
	"io"
	"os"
	"strings"

	"github.com/trickstertwo/xlog"
)

// Register this adapter as the default for xlog.Default()/New().
// Format can be controlled with XLOG_ADAPTER_FORMAT=JSON|TEXT (case-insensitive).
func init() {
	xlog.RegisterDefaultAdapterFactory(func(w io.Writer) xlog.Adapter {
		format := FormatText
		switch f := strings.ToLower(os.Getenv("XLOG_ADAPTER_FORMAT")); f {
		case "json":
			format = FormatJSON
		case "text", "":
			format = FormatText
		}
		if w == nil {
			w = os.Stdout
		}
		return New(w, Options{Format: format})
	})
}
