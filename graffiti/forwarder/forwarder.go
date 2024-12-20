/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package forwarder

import (
    "os"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// Forwarder forwards the topology to only one master server.
// When switching from one analyzer to another one the agent does a full
// re-sync since some messages could have been lost.
type Forwarder struct {
	masterElection *ws.MasterElection
	graph          *graph.Graph
	logger         logging.Logger
}

func (t *Forwarder) triggerResync() {
	t.logger.Infof("Start a re-sync")

	// re-add all the nodes and edges
	msg := &messages.SyncMsg{
		Elements: t.graph.Elements(),
	}
	t.masterElection.SendMessageToMaster(messages.NewStructMessage(messages.SyncMsgType, msg))
}

// OnNewMaster is called by the master election mechanism when a new master is elected. In
// such case a "Re-sync" is triggered in order to be in sync with the new master.
func (t *Forwarder) OnNewMaster(c ws.Speaker) {
	if c == nil {
		t.logger.Warning("Lost connection to master")

		// do not forward message before re-sync
		t.graph.RemoveEventListener(t)

		os.Exit(1)
	} else {
		addr, port := c.GetAddrPort()
		t.logger.Infof("Using %s:%d as master of topology forwarder", addr, port)

		t.graph.RLock()

		t.triggerResync()

		// synced can now listen the graph
		t.graph.AddEventListener(t)

		t.graph.RUnlock()
	}
}

// OnNodeUpdated graph node updated event. Implements the EventListener interface.
func (t *Forwarder) OnNodeUpdated(n *graph.Node, ops []graph.PartiallyUpdatedOp) {
	t.masterElection.SendMessageToMaster(
		messages.NewStructMessage(
			messages.NodePartiallyUpdatedMsgType,
			messages.PartiallyUpdatedMsg{
				ID:        n.ID,
				UpdatedAt: n.UpdatedAt,
				Revision:  n.Revision,
				Ops:       ops,
			},
		),
	)
}

// OnNodeAdded graph node added event. Implements the EventListener interface.
func (t *Forwarder) OnNodeAdded(n *graph.Node) {
	t.masterElection.SendMessageToMaster(
		messages.NewStructMessage(messages.NodeAddedMsgType, n),
	)
}

// OnNodeDeleted graph node deleted event. Implements the EventListener interface.
func (t *Forwarder) OnNodeDeleted(n *graph.Node) {
	t.masterElection.SendMessageToMaster(
		messages.NewStructMessage(messages.NodeDeletedMsgType, n),
	)
}

// OnEdgeUpdated graph edge updated event. Implements the EventListener interface.
func (t *Forwarder) OnEdgeUpdated(e *graph.Edge, ops []graph.PartiallyUpdatedOp) {
	t.masterElection.SendMessageToMaster(
		messages.NewStructMessage(
			messages.EdgePartiallyUpdatedMsgType,
			messages.PartiallyUpdatedMsg{
				ID:        e.ID,
				UpdatedAt: e.UpdatedAt,
				Revision:  e.Revision,
				Ops:       ops,
			},
		),
	)
}

// OnEdgeAdded graph edge added event. Implements the EventListener interface.
func (t *Forwarder) OnEdgeAdded(e *graph.Edge) {
	t.masterElection.SendMessageToMaster(
		messages.NewStructMessage(messages.EdgeAddedMsgType, e),
	)
}

// OnEdgeDeleted graph edge deleted event. Implements the EventListener interface.
func (t *Forwarder) OnEdgeDeleted(e *graph.Edge) {
	t.masterElection.SendMessageToMaster(
		messages.NewStructMessage(messages.EdgeDeletedMsgType, e),
	)
}

// GetMaster returns the current analyzer the agent is sending its events to
func (t *Forwarder) GetMaster() ws.Speaker {
	return t.masterElection.GetMaster()
}

// NewForwarder returns a new Graph forwarder which forwards event of the given graph
// to the given WebSocket JSON speakers.
func NewForwarder(g *graph.Graph, pool ws.StructSpeakerPool, logger logging.Logger) *Forwarder {
	if logger == nil {
		logger = logging.GetLogger()
	}

	masterElection := ws.NewMasterElection(pool)

	t := &Forwarder{
		masterElection: masterElection,
		graph:          g,
		logger:         logger,
	}

	masterElection.AddEventHandler(t)

	return t
}
