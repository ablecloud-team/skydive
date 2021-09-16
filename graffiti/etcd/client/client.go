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

package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	etcd "go.etcd.io/etcd/client/v2"

	"github.com/skydive-project/skydive/graffiti/logging"
)

// Client defaults
const (
	DefaultTimeout = 5 * time.Second
	DefaultPort    = 12379
	DefaultServer  = "127.0.0.1"
)

// MasterElectionListener describes the multi election mechanism
type MasterElectionListener interface {
	OnStartAsMaster()
	OnStartAsSlave()
	OnSwitchToMaster()
	OnSwitchToSlave()
}

// MasterElection describes the master election mechanism
type MasterElection interface {
	Start()
	StartAndWait()
	Stop()
	IsMaster() bool
	AddEventListener(listener MasterElectionListener)
	TTL() time.Duration
}

// MasterElectionService describes the election service mechanism
type MasterElectionService interface {
	NewElection(key string) MasterElection
}

// Client describes a ETCD configuration client
type Client struct {
	id      string
	client  *etcd.Client
	KeysAPI etcd.KeysAPI
	logger  logging.Logger
}

// Opts describes the options of an etcd client
type Opts struct {
	Servers []string
	Timeout time.Duration
	Logger  logging.Logger
}

// GetInt64 returns an int64 value from the configuration key
func (client *Client) GetInt64(key string) (int64, error) {
	resp, err := client.KeysAPI.Get(context.Background(), key, nil)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(resp.Node.Value, 10, 64)
}

// SetInt64 set an int64 value to the configuration key
func (client *Client) SetInt64(key string, value int64) error {
	_, err := client.KeysAPI.Set(context.Background(), key, strconv.FormatInt(value, 10), nil)
	return err
}

// Start the client
func (client *Client) Start() {
	// wait for etcd to be ready
	for {
		if err := client.SetInt64(fmt.Sprintf("/client:%s/start-time", client.id), time.Now().Unix()); err != nil {
			client.logger.Errorf("Etcd server not ready: %s", err)
			time.Sleep(time.Second)
		} else {
			break
		}
	}
}

// Stop the client
func (client *Client) Stop() {
	if tr, ok := etcd.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

// NewElection creates a new ETCD master elector
func (client *Client) NewElection(path string) MasterElection {
	return NewMasterElector(client, path)
}

// NewClient creates a new ETCD client connection to ETCD servers
func NewClient(id string, opts Opts) (*Client, error) {
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}

	if len(opts.Servers) == 0 {
		opts.Servers = []string{fmt.Sprintf("%s:%d", DefaultServer, DefaultPort)}
	}

	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	cfg := etcd.Config{
		Endpoints:               opts.Servers,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: opts.Timeout,
	}

	client, err := etcd.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to etcd: %s", err)
	}

	kapi := etcd.NewKeysAPI(client)

	return &Client{
		id:      id,
		client:  &client,
		KeysAPI: kapi,
		logger:  opts.Logger,
	}, nil
}
