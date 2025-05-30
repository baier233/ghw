//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// PackFrom creates the snapshot named `snapshotName` from the
// directory tree whose root is `sourceRoot`.
func PackFrom(snapshotName, sourceRoot string) error {
	f, err := OpenDestination(snapshotName)
	if err != nil {
		return err
	}
	defer f.Close()

	return PackWithWriter(f, sourceRoot)
}

// OpenDestination opens the `snapshotName` file for writing, bailing out
// if the file seems to exist and have existing content already.
// This is done to avoid accidental overwrites.
func OpenDestination(snapshotName string) (*os.File, error) {
	f, err := os.OpenFile(snapshotName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		fs, err := os.Stat(snapshotName)
		if err != nil {
			return nil, err
		}
		if fs.Size() > 0 {
			return nil, fmt.Errorf("file %s already exists and is of size > 0", snapshotName)
		}
		f, err = os.OpenFile(snapshotName, os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

// PakcWithWriter creates a snapshot sending all the binary data to the
// given `fw` writer. The snapshot is made from the directory tree whose
// root is `sourceRoot`.
func PackWithWriter(fw io.Writer, sourceRoot string) error {
	gzw := gzip.NewWriter(fw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return createSnapshot(tw, sourceRoot)
}

func createSnapshot(tw *tar.Writer, buildDir string) error {
	return filepath.Walk(buildDir, func(path string, fi os.FileInfo, _ error) error {
		if path == buildDir {
			return nil
		}
		var link string
		var err error

		if fi.Mode()&os.ModeSymlink != 0 {
			trace("processing symlink %s\n", path)
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(buildDir, path)
		if err != nil {
			return err
		}
		hdr.Name = relPath

		if err = tw.WriteHeader(hdr); err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err = io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
}
