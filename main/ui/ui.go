// Copyright 2015 Square Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/square/metrics/log"

	// "github.com/square/metrics/api"
	"github.com/square/metrics/api/backend"
	"github.com/square/metrics/metric_metadata/cassandra"
	"github.com/square/metrics/timeseries_storage/blueflood"
	// "github.com/square/metrics/api/backend/blueflood"
	"github.com/square/metrics/function/registry"
	"github.com/square/metrics/main/common"
	"github.com/square/metrics/query"
	"github.com/square/metrics/ui"
	"github.com/square/metrics/util"
)

func startServer(config common.UIConfig, context query.ExecutionContext) {
	httpMux := ui.NewMux(config.Config, context, ui.Hook{})

	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", config.Port),
		Handler:        httpMux,
		ReadTimeout:    time.Duration(config.Timeout) * time.Second,
		WriteTimeout:   time.Duration(config.Timeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	err := server.ListenAndServe()
	if err != nil {
		log.Infof(err.Error())
	}
}

func main() {
	flag.Parse()
	common.SetupLogger()

	config := common.LoadConfig()

	cassandraConfig := cassandra.CassandraMetricMetadataConfig{
		Hosts:    config.MetricMetadataConfig.Hosts,
		Keyspace: config.MetricMetadataConfig.Keyspace,
	}
	apiInstance := common.NewMetricMetadataAPI(cassandraConfig)

	ruleset, err := util.LoadRules(config.MetricMetadataConfig.ConversionRulesPath)
	if err != nil {
		//Blah
	}
	graphite := util.RuleBasedGraphiteConverter{Ruleset: ruleset}
	config.Blueflood.GraphiteMetricConverter = &graphite

	blueflood := blueflood.NewBlueflood(config.Blueflood)

	backend := backend.NewParallelMultiBackend(blueflood, 20)

	startServer(config.UIConfig, query.ExecutionContext{
		MetricMetadataAPI: apiInstance,
		MultiBackend:      *backend,
		FetchLimit:        1000,
		SlotLimit:         5000,
		Registry:          registry.Default(),
	})
}
