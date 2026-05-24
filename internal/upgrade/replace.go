package upgrade

import "fmt"

type ManualReplaceError struct {
	TempPath string
	ExePath  string
}

func (e *ManualReplaceError) Error() string {
	return fmt.Sprintf("manual replacement required: move /Y %q %q", e.TempPath, e.ExePath)
}
