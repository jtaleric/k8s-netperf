package metrics

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gihub.com/jtaleric/k8s-netperf/logging"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type NodeCPU struct {
	Idle    float64
	User    float64
	Steal   float64
	System  float64
	Nice    float64
	Irq     float64
	Softirq float64
	Iowait  float64
}

type PodCPU struct {
	Name  string
	Value float64
}

type PodValues struct {
	Results []PodCPU
}

func PromCheck(url string) bool {
	_, ret := promQueryRange(time.Now(), time.Now(), "prometheus_build_info", url)
	return ret
}

func QueryNodeCPU(node string, url string, start time.Time, end time.Time) (NodeCPU, bool) {
	cpu := NodeCPU{}
	query := fmt.Sprintf("(avg by(mode) (rate(node_cpu_seconds_total{instance=~\"%s:.*\"}[1m])) * 100)", node)
	val, q := promQueryRange(start, end, query, url)
	if !q {
		logging.Error("Issue querying Prometheus")
		return cpu, false
	}
	if val.Type() == model.ValMatrix {
		v := val.(model.Matrix)
		for _, s := range v {
			if strings.Contains(s.Metric.String(), "idle") {
				cpu.Idle = avg(s.Values)
			}
			if strings.Contains(s.Metric.String(), "steal") {
				cpu.Steal = avg(s.Values)
			}
			if strings.Contains(s.Metric.String(), "system") {
				cpu.System = avg(s.Values)
			}
			if strings.Contains(s.Metric.String(), "user") {
				cpu.User = avg(s.Values)
			}
			if strings.Contains(s.Metric.String(), "nice") {
				cpu.Nice = avg(s.Values)
			}
			if strings.Contains(s.Metric.String(), "\"irq\"") {
				cpu.Irq = avg(s.Values)
			}
			if strings.Contains(s.Metric.String(), "softirq") {
				cpu.Softirq = avg(s.Values)
			}
			if strings.Contains(s.Metric.String(), "iowait") {
				cpu.Iowait = avg(s.Values)
			}
		}

	}
	return cpu, true
}

func TopPodCPU(node string, url string, start time.Time, end time.Time) (PodValues, bool) {
	var pods PodValues
	re := regexp.MustCompile("pod=\"(.*)\"")
	query := fmt.Sprintf("topk(5,sum(irate(container_cpu_usage_seconds_total{name!=\"\",instance=~\"%s:.*\"}[2m]) * 100) by (pod, namespace, instance))", node)
	val, q := promQueryRange(start, end, query, url)
	if !q {
		logging.Error("Issue querying Prometheus")
		return pods, false
	}
	if val.Type() == model.ValMatrix {
		v := val.(model.Matrix)
		for _, s := range v {
			p := PodCPU{
				Name:  re.FindStringSubmatch(s.Metric.String())[1],
				Value: avg(s.Values),
			}
			pods.Results = append(pods.Results, p)
		}
	}
	return pods, true
}

func avg(data []model.SamplePair) float64 {
	sum := 0.0
	for s := range data {
		sum += float64(data[s].Value)
	}
	return sum / float64(len(data))
}

func promQueryRange(start time.Time, end time.Time, query string, url string) (model.Value, bool) {
	client, err := api.NewClient(api.Config{
		Address: url,
	})
	if err != nil {
		logging.Errorf("Error creating client: %v\n", err)
		return nil, false
	}
	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := v1.Range{
		Start: start,
		End:   end,
		Step:  time.Minute,
	}
	result, warnings, err := v1api.QueryRange(ctx, query, r, v1.WithTimeout(5*time.Second))
	if err != nil {
		logging.Errorf("Error querying Prometheus: %v\n", err)
		return nil, false
	}
	if len(warnings) > 0 {
		logging.Warnf("Warnings: %v\n", warnings)
	}
	return result, true
}
