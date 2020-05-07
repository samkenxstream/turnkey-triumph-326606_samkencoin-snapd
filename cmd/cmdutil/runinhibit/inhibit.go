// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2020 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

// Package runinhibit contains operations for establishing, removing and
// querying snap run inhibition lock.
package runinhibit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/snapcore/snapd/dirs"
	"github.com/snapcore/snapd/osutil"
)

// defaultInhibitDir is the directory where inhibition files are stored.
const defaultInhibitDir = "/var/lib/snapd/inhibit"

// InhibitDir is the directory where inhibition files are stored.
// This value can be changed by calling dirs.SetRootDir.
var InhibitDir = defaultInhibitDir

func init() {
	dirs.AddRootDirCallback(func(root string) {
		InhibitDir = filepath.Join(root, defaultInhibitDir)
	})
}

// Hint is a string representing reason for the inhibition of "snap run".
type Hint string

const (
	// HintNotInhibited is used when "snap run" is not inhibited.
	HintNotInhibited Hint = ""
	// HintInhibitedForRefresh represents inhibition of a "snap run" while a refresh change is being performed.
	HintInhibitedForRefresh Hint = "refresh"
)

func openHintFile(snapName string) (*osutil.FileLock, error) {
	fname := filepath.Join(InhibitDir, snapName+".lock")
	return osutil.NewFileLockWithMode(fname, 0644)
}

// LockWithHint sets a persistent "snap run" inhibition lock with a given hint.
//
// The hint cannot be empty. It should be one of the Hint constants defined in
// this package. While the hint in place "snap run" will not allow the snap to
// start and will block, presenting a user interface if possible.
func LockWithHint(snapName string, hint Hint) error {
	if len(hint) == 0 {
		return fmt.Errorf("hint cannot be empty")
	}
	if err := os.MkdirAll(InhibitDir, 0755); err != nil {
		return err
	}
	flock, err := openHintFile(snapName)
	if err != nil {
		return err
	}
	defer flock.Close()

	if err := flock.Lock(); err != nil {
		return err
	}
	f := flock.File()
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err = f.WriteString(string(hint))
	return err
}

// Unlock truncates the run inhibition lock.
//
// Truncated inhibition lock is equivalent to uninhibited "snap run".
func Unlock(snapName string) error {
	flock, err := openHintFile(snapName)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer flock.Close()

	if err := flock.Lock(); err != nil {
		return err
	}
	f := flock.File()
	return f.Truncate(0)
}

// IsLocked returns information about the run inhibition hint, if any.
func IsLocked(snapName string) (Hint, error) {
	flock, err := openHintFile(snapName)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer flock.Close()

	if err := flock.ReadLock(); err != nil {
		return "", err
	}

	f := flock.File()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	if fi.Size() == 0 {
		return "", nil
	}

	buf := make([]byte, fi.Size())
	n, err := f.Read(buf)
	if n == len(buf) {
		return Hint(string(buf)), nil
	}
	return "", err
}
