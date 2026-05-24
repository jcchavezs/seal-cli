//go:build windows

package upgrade

import "os"

func ReplaceExecutable(exe string, data []byte) error {
	tmp := exe + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	return &ManualReplaceError{TempPath: tmp, ExePath: exe}
}
