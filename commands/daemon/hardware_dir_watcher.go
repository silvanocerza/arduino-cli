// This file is part of arduino-cli.
//
// Copyright 2020 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package daemon

import (
	"fmt"

	"github.com/arduino/arduino-cli/configuration"
	"github.com/arduino/go-paths-helper"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// HardwareDirWatcher has the job to notify of changes in the Hardware folder
// of the current user' Sketchbook, specified by the "directories.user" setting.
// Ideally this watcher is meant to be used when the CLI is run in daemon mode, though if the
// user changes the "directories.user" setting the watcher doesn't update the folder it's watching
// and keeps sending notifications when the old folder is modified.
type HardwareDirWatcher struct {
	hardwareDir *paths.Path
	changesChan chan fsnotify.Op
	fsWatcher   *fsnotify.Watcher
}

// NewHardwareDirWatcher creates a new instance of hardwareDirWatcher
// and returns it fully configured ready to be started.
func NewHardwareDirWatcher() (*HardwareDirWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	userDir := paths.New(configuration.Settings.GetString("directories.user"))
	if userDir.NotExist() {
		return nil, fmt.Errorf("directories.user path is not set")
	}

	hardwareDir := userDir.Join("hardware")
	if hardwareDir.NotExist() {
		if err := hardwareDir.MkdirAll(); err != nil {
			return nil, fmt.Errorf("creating directory: %v", err)
		}
	}

	if err := fsWatcher.Add(hardwareDir.String()); err != nil {
		return nil, err
	}

	dirWatcher := &HardwareDirWatcher{
		hardwareDir: hardwareDir,
		changesChan: make(chan fsnotify.Op),
		fsWatcher:   fsWatcher,
	}

	return dirWatcher, nil
}

// ChangesChannel returns the channel used to monitor for changes in
// the monitored directory.
func (s *HardwareDirWatcher) ChangesChannel() <-chan fsnotify.Op {
	return s.changesChan
}

// Start the watcher loop and waits for changes in the monitor channels.
// Stops automatically in case of errors.
func (s *HardwareDirWatcher) Start() {
	logrus.Info("starting hardware dir watcher")
	go func() {
		defer s.fsWatcher.Close()
		for {
			select {
			case events, ok := <-s.fsWatcher.Events:
				if !ok {
					logrus.Info("stopping hardware watcher events")
					return
				}
				s.changesChan <- events.Op
			case err, ok := <-s.fsWatcher.Errors:
				if !ok {
					logrus.Info("stopping hardware watcher errors")
					return
				}
				logrus.Fatalf("watching hardware folder: %v", err)
			}
		}
	}()
}

// Stop the file watcher.
func (s *HardwareDirWatcher) Stop() error {
	logrus.Info("stopping hardware dir watcher")
	return s.fsWatcher.Close()
}
