package collector

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	chainLabelNames = []string{"chain", "rule"}
)

type iptablesCollector struct {
	chainDownloadDesc *prometheus.Desc
	chainUploadDesc   *prometheus.Desc
	logger            log.Logger
}

func init() {
	registerCollector("iptables", defaultDisabled, NewIptablesCollector)
}

// NewIptablesCollector returns a new Collector exposing iptables stats.
func NewIptablesCollector(logger log.Logger) (Collector, error) {
	subsystem := "iptables"
	chainDownloadDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "download_bytes"),
		"Iptables download traffic in each chains.",
		chainLabelNames, nil,
	)
	chainUploadDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "upload_bytes"),
		"Iptables upload traffic in each chains.",
		chainLabelNames, nil,
	)

	return &iptablesCollector{
		chainDownloadDesc: chainDownloadDesc,
		chainUploadDesc:   chainUploadDesc,
		logger:            logger,
	}, nil
}

func (c *iptablesCollector) Update(ch chan<- prometheus.Metric) error {
	out, err := exec.Command("iptables", "-nxvL").Output()
	// out, err := exec.Command("cat", "test").Output()
	if err != nil {
		return err
	}

	currentChain := "UNKNOWN"
	downloadRe := regexp.MustCompile(`/\* DOWNLOAD (.*) \*/`)
	uploadRe := regexp.MustCompile(`/\* UPLOAD (.*) \*/`)
	for _, line := range strings.Split(string(out), "\n") {
		rawData := strings.Fields(line)
		if len(rawData) <= 0 {
			continue
		}
		if rawData[0] == "Chain" {
			currentChain = rawData[1]
		}
		if _, err := strconv.Atoi(rawData[0]); err != nil {
			continue
		}
		if n, err := strconv.ParseFloat(rawData[1], 64); err == nil {
			if downloadMatch := downloadRe.FindStringSubmatch(line); len(downloadMatch) > 1 {
				ch <- prometheus.MustNewConstMetric(
					c.chainDownloadDesc, prometheus.GaugeValue,
					n, currentChain, downloadMatch[1],
				)
			} else if uploadMatch := uploadRe.FindStringSubmatch(line); len(uploadMatch) > 1 {
				ch <- prometheus.MustNewConstMetric(
					c.chainDownloadDesc, prometheus.GaugeValue,
					n, currentChain, uploadMatch[1],
				)
			}
		}
	}
	return nil
}
