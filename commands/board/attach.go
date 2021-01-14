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

package board

import (
	"context"
	"errors"
	"fmt"

	"github.com/arduino/arduino-cli/arduino/cores"
	"github.com/arduino/arduino-cli/arduino/sketches"
	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/commands"
	"github.com/arduino/go-paths-helper"
)

// Attach FIXMEDOC
func Attach(ctx context.Context, req *rpc.BoardAttachReq, taskCB commands.TaskProgressCB) (*rpc.BoardAttachResp, error) {
	var sketchPath *paths.Path
	if req.GetSketchPath() != "" {
		sketchPath = paths.New(req.GetSketchPath())
	}
	sketch, err := sketches.NewSketchFromPath(sketchPath)
	if err != nil {
		return nil, fmt.Errorf("opening sketch: %s", err)
	}

	if req.Fqbn != "" && req.Address != "" {
		pm := commands.GetPackageManager(req.Instance.Id)
		if pm == nil {
			return nil, errors.New("invalid instance")
		}

		fqbn, err := cores.ParseFQBN(req.Fqbn)
		if err != nil {
			return nil, fmt.Errorf("parsing fqbn: %w", err)
		}

		boardName := fqbn.BoardID
		board, err := pm.FindBoardWithFQBN(fqbn.String())
		// If this point is reached we already know the FQBN is formatted correctly, so the only
		// possible error is a board not found.
		// The board is needed only to get the name.
		if err == nil {
			boardName = board.Name()
		}

		sketch.Metadata.CPU.Fqbn = fqbn.String()
		sketch.Metadata.CPU.Name = boardName
		sketch.Metadata.CPU.Port = req.Address
	} else if req.Fqbn != "" {
		fqbn, err := cores.ParseFQBN(req.Fqbn)
		if err != nil {
			return nil, fmt.Errorf("parsing fqbn: %w", err)
		}

		ports, err := List(req.Instance.Id)
		if err != nil {
			return nil, fmt.Errorf("detecting boards: %w", err)
		}

		// Sort ports so that serial connection have priority
		sortedPorts := []*rpc.DetectedPort{}
		protocols := map[string]bool{
			"tty":    true,
			"serial": true,
			"http":   false,
			"https":  false,
			"tcp":    false,
			"udp":    false,
		}
		for _, p := range ports {
			if protocols[p.Protocol] {
				sortedPorts = append([]*rpc.DetectedPort{p}, sortedPorts...)
			} else {
				sortedPorts = append(sortedPorts, p)
			}
		}

		for _, port := range sortedPorts {
			for _, board := range port.Boards {
				if board.FQBN == fqbn.String() {
					sketch.Metadata.CPU.Fqbn = board.FQBN
					sketch.Metadata.CPU.Name = board.Name
					sketch.Metadata.CPU.Port = port.Address
					goto done
				}
			}
		}

	} else if req.Address != "" {
		ports, err := List(req.Instance.Id)
		if err != nil {
			return nil, fmt.Errorf("detecting boards: %w", err)
		}

		for _, port := range ports {
			if port.Address == req.Address {
				// There can be multiple boards possible boards in a port if it couldn't be identified
				// with certainty for any reason.
				if len(port.Boards) > 0 {
					board := port.Boards[0]
					sketch.Metadata.CPU.Fqbn = board.FQBN
					sketch.Metadata.CPU.Name = board.Name
					sketch.Metadata.CPU.Port = port.Address
					goto done
				}
			}
		}
	} else {
		return nil, errors.New("")
	}

done:
	err = sketch.ExportMetadata()
	if err != nil {
		return nil, fmt.Errorf("cannot export sketch metadata: %s", err)
	}
	taskCB(&rpc.TaskProgress{Name: "Selected fqbn: " + sketch.Metadata.CPU.Fqbn, Completed: true})
	return &rpc.BoardAttachResp{}, nil
}
