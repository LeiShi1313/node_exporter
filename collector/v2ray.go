package collector

import (
	"context"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"gopkg.in/alecthomas/kingpin.v2"
	"v2ray.com/core/app/stats/command"
)

var (
	endpoint = kingpin.Flag(
		"collector.v2ray.endpoint",
		"V2ray api endpoint.",
	).Default("127.0.0.1:10085").String()
	timeout = kingpin.Flag(
		"collector.v2ray.timeout",
		"V2ray api timeout in seconds.",
	).Default("3").Uint8()
	v2rayLabelNames = []string{"dimension", "target"}
)

type v2rayCollector struct {
	upDesc                 *prometheus.Desc
	uptimeDesc             *prometheus.Desc
	downloadBytesTotalDesc *prometheus.Desc
	uploadBytesTotalDesc   *prometheus.Desc
	logger                 log.Logger
}

func init() {
	registerCollector("v2ray", defaultDisabled, NewV2rayCollector)
}

// NewV2rayCollector returns a new Collecotr exposing v2ray stats.
func NewV2rayCollector(logger log.Logger) (Collector, error) {
	subsystem := "v2ray"
	upDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "up"),
		"Indicate whether there is an V2ray instance running or not.",
		nil, nil,
	)
	uptimeDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "uptime"),
		"V2Ray uptime in seconds.",
		nil, nil,
	)
	downloadBytesTotalDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "download_bytes_total"),
		"Number of downloaded bytes.",
		v2rayLabelNames, nil,
	)
	uploadBytesTotalDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "upload_bytes_total"),
		"Number of uploaded bytes.",
		v2rayLabelNames, nil,
	)

	return &v2rayCollector{
		upDesc:                 upDesc,
		uptimeDesc:             uptimeDesc,
		downloadBytesTotalDesc: downloadBytesTotalDesc,
		uploadBytesTotalDesc:   uploadBytesTotalDesc,
		logger:                 logger,
	}, nil
}

func (c *v2rayCollector) Update(ch chan<- prometheus.Metric) error {
	if err := c.collectV2rayMetrics(ch); err != nil {
		ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, 0.0)
		return err
	}

	return nil
}

func (c *v2rayCollector) collectV2rayMetrics(ch chan<- prometheus.Metric) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, *endpoint, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := command.NewStatsServiceClient(conn)
	if err := c.collectV2RaySysMetrics(ctx, ch, client); err != nil {
		return err
	}
	if err := c.collectV2RayLevelsMetrics(ctx, ch, client); err != nil {
		return err
	}

	return nil
}

func (c *v2rayCollector) collectV2RaySysMetrics(ctx context.Context, ch chan<- prometheus.Metric, client command.StatsServiceClient) error {
	resp, err := client.GetSysStats(ctx, &command.SysStatsRequest{})
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.uptimeDesc, prometheus.GaugeValue, float64(resp.GetUptime()))
	ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, 1.0)

	return nil
}

func (c *v2rayCollector) collectV2RayLevelsMetrics(ctx context.Context, ch chan<- prometheus.Metric, client command.StatsServiceClient) error {
	resp, err := client.QueryStats(ctx, &command.QueryStatsRequest{Reset_: false})
	if err != nil {
		return err
	}

	for _, s := range resp.GetStat() {
		// example value: inbound>>>socks-proxy>>>traffic>>>uplink
		p := strings.Split(s.GetName(), ">>>")
		dimension := p[0]
		target := p[1]
		if p[3] == "uplink" {
			ch <- prometheus.MustNewConstMetric(c.uploadBytesTotalDesc, prometheus.GaugeValue, float64(s.GetValue()), dimension, target)
		} else {
			ch <- prometheus.MustNewConstMetric(c.downloadBytesTotalDesc, prometheus.GaugeValue, float64(s.GetValue()), dimension, target)
		}
	}

	return nil
}
