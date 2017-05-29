/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"strconv"
	"time"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	v3 "google.golang.org/api/monitoring/v3"

	"k8s.io/contrib/prometheus-to-sd/config"
	"k8s.io/contrib/prometheus-to-sd/flags"
	"k8s.io/contrib/prometheus-to-sd/translator"
)

var (
	host       = flag.String("target-host", "localhost", "The monitored component's hostname.")
	port       = flag.Uint("target-port", 80, "The monitored component's port.")
	component  = flag.String("component", "", "The monitored target's name. DEPRECATED: Use --source instead.")
	resolution = flag.Duration("metrics-resolution", 60*time.Second,
		"The resolution at which prometheus-to-sd will scrape the component for metrics.")
	metricsPrefix = flag.String("stackdriver-prefix", "container.googleapis.com/master",
		"Prefix that is appended to every metric.")
	whitelisted = flag.String("whitelisted-metrics", "",
		"Comma-separated list of whitelisted metrics. If empty all metrics will be exported.")
	apioverride = flag.String("api-override", "",
		"The stackdriver API endpoint to override the default one used (which is prod).")
	source = flags.Uris{}
)

func main() {
	flag.Set("logtostderr", "true")
	flag.Var(&source, "source", "source(s) to watch in [component]:http://host:port?whitelisted=a,b,c format")

	defer glog.Flush()
	flag.Parse()

	var sourceConfigs []config.SourceConfig

	for _, c := range source {
		if sourceConfig, err := config.ParseSourceConfig(c); err != nil {
			glog.Fatalf("Error while parsing source config flag %v: %v", c, err)
		} else {
			sourceConfigs = append(sourceConfigs, *sourceConfig)
		}
	}

	if *component != "" {
		portStr := strconv.FormatUint(uint64(*port), 10)

		if sourceConfig, err := config.NewSourceConfig(*component, *host, portStr, *whitelisted); err != nil {
			glog.Fatalf("Error while parsing --component flag: %v", err)
		} else {
			glog.Infof("--component flag was specified. Created a new source instance: %v", sourceConfig)
			sourceConfigs = append(sourceConfigs, *sourceConfig)
		}
	}

	gceConf, err := config.GetGceConfig(*metricsPrefix)
	if err != nil {
		glog.Fatalf("Failed to get GCE config: %v", err)
	}
	glog.Infof("GCE config: %+v", gceConf)

	client := oauth2.NewClient(oauth2.NoContext, google.ComputeTokenSource(""))
	stackdriverService, err := v3.New(client)
	if *apioverride != "" {
		stackdriverService.BasePath = *apioverride
	}
	if err != nil {
		glog.Fatalf("Failed to create Stackdriver client: %v", err)
	}
	glog.V(4).Infof("Successfully created Stackdriver client")

	if len(sourceConfigs) == 0 {
		glog.Fatalf("No sources defined. Please specify at least one --source flag.")
	}

	for _, sourceConfig := range sourceConfigs {
		glog.V(4).Infof("Starting goroutine for %v", sourceConfig)

		// Pass sourceConfig as a parameter to avoid using the last sourceConfig by all goroutines.
		go func(sourceConfig config.SourceConfig) {
			glog.Infof("Running prometheus-to-sd, monitored target is %s %v:%v", sourceConfig.Component, sourceConfig.Host, sourceConfig.Port)

			for range time.Tick(*resolution) {
				glog.V(4).Infof("Scraping metrics")
				metrics, err := translator.GetPrometheusMetrics(sourceConfig.Host, sourceConfig.Port)
				if err != nil {
					glog.Warningf("Error while getting Prometheus metrics %v", err)
					continue
				}

				ts := translator.TranslatePrometheusToStackdriver(gceConf, sourceConfig.Component, metrics, sourceConfig.Whitelisted)
				translator.SendToStackdriver(stackdriverService, gceConf, ts)
			}
		}(sourceConfig)
	}

	// As worker goroutines work forever, block main thread as well.
	<-make(chan int)
}
