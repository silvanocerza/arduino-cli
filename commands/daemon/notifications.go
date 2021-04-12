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
	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/notifications/v1"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// NotificationsService implements the `Notifications` service
type NotificationsService struct {
	notificationsChan chan rpc.Notification

	hardwareDirWatcher *HardwareDirWatcher
}

// NewNotificationService creates a new NotificationService.
// This service is used when the CLI is run in daemon mode to send notifications
// to connected gRPC clients.
func NewNotificationService() (*NotificationsService, error) {
	service := &NotificationsService{
		notificationsChan: make(chan rpc.Notification),
	}

	var err error
	// Creates a new hardware dir watcher, if a gRPC client changes
	// the "directories.user" config the daemon must be reloaded otherwise
	// changes on the new folder won't be received.
	service.hardwareDirWatcher, err = NewHardwareDirWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for op := range service.hardwareDirWatcher.ChangesChannel() {
			switch op {
			case fsnotify.Create:
				fallthrough
			case fsnotify.Write:
				fallthrough
			case fsnotify.Remove:
				fallthrough
			case fsnotify.Rename:
				logrus.Info("manually installed cores change detected")
				// Updates all existing instances
				for _, id := range commands.GetInstancesIds() {
					commands.Rescan(id)
				}
				service.notificationsChan <- rpc.Notification_NOTIFICATION_CORE_CHANGED
			}
		}
	}()
	service.hardwareDirWatcher.Start()
	return service, nil
}

// GetNotifications sends notifications to connected gRPC clients
func (s *NotificationsService) GetNotifications(res *rpc.GetNotificationsRequest, stream rpc.NotificationsService_GetNotificationsServer) error {
	filter := map[rpc.Notification]bool{}
	for _, f := range res.Filter {
		filter[f] = true
	}

	for notification := range s.notificationsChan {
		// Filter out notifications, if filter is empty everything is accepted
		if len(filter) != 0 && !filter[notification] {
			logrus.Infof("skipped notification %s", notification)
			continue
		}

		logrus.Infof("Sending notification %s", notification)

		if err := stream.Send(&rpc.GetNotificationsResponse{Notification: notification}); err != nil {
			logrus.Info("stopping this shit")
			s.hardwareDirWatcher.Stop()
			return err
		}
	}
	return nil
}
