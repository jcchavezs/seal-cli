package seal

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// WalkRegularFiles invokes fn once per regular file under root. Foundation
// for HashSealedRoot.
//
// fn receives:
//   - rel: sealed-root-relative forward-slash path (the files-map key form)
//   - full: host filesystem path, ready for os.Open
//
// fn returning an error aborts the walk; the error propagates unchanged.
//
// Exclusions:
//   - .git (directory or file) at any depth - version-control metadata
//     changes on every git op and would churn the lockfile.
//
// Symlinks and other non-regular files error out. Pin aborts; verify
// surfaces this to the classifier which marks the bundle as Mismatch.
func WalkRegularFiles(root string, fn func(rel, full string) error) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Prune the .git subtree; SkipDir is cheaper than visiting every entry.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		// Bare .git file (git uses these as submodule pointers).
		if d.Name() == ".git" {
			return nil
		}

		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		// Forward slashes: downstream NFC/sort/hash code expects them, and
		// they're what ends up in files-map keys.
		rel = filepath.ToSlash(rel)

		// d.Type() excludes the regular-file marker; a regular file is 0.
		// Anything else (symlink, device, socket, pipe, irregular) errors -
		// no silent skip.
		if d.Type()&fs.ModeType != 0 {
			return fmt.Errorf("unsupported file type at %s: %s", p, d.Type())
		}

		return fn(rel, p)
	})
}
