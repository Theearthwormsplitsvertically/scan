// Package container 实现容器与镜像资产扫描模块。
package container

import (
	"context"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	hostmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/host"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
	"github.com/Theearthwormsplitsvertically/scan/internal/security"
)

// Collector 是容器模块的无状态实现。
type Collector struct{}

// New 创建容器模块。
func New() Collector { return Collector{} }

// Descriptor 返回容器模块的依赖、周期和资源约束。
// host 是硬依赖（稳定归属）；process 与 port 是软依赖（cgroup 归属、宿主端口监听校验，失败不阻断）。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "container", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes:          []string{"container", "container_image"},
		RequiredCapabilities: []string{provider.CapabilityContainer}, OptionalCapabilities: []string{},
		HardDependencies: []string{"host"}, SoftDependencies: []string{"process", "port"},
		DefaultInterval: "12h", ResourceClass: "medium", Timeout: "2m",
	}
}

// Probe 判断当前平台是否提供强类型 container 能力。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.ContainerProvider](providers, provider.CapabilityContainer); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported, Reason: "当前平台未提供 container Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 container Provider", Provider: provider.CapabilityContainer}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 发布 container 与 container_image 记录，并生成 based_on、runs_on 与 runs_in 关系。
func (Collector) Collect(ctx context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	host, ok := hostDependency(request)
	if !ok {
		return failedResult(started, model.StatusFailed, "缺少 host 硬依赖结果")
	}
	backend, ok := provider.As[provider.ContainerProvider](providers, provider.CapabilityContainer)
	if !ok {
		return failedResult(started, model.StatusUnsupported, "缺少 container Provider")
	}
	containers, images, status := backend.Collect(ctx)
	hostID := hostmodule.RecordID(host)
	processes := processFacts(request) // 软依赖，缺失时不生成 runs_in
	pidRecordIDs := processPIDRecordIDs(request)
	listeningPorts, portAvailable := listeningHostPorts(request) // 软依赖 port，缺失时不生成监听校验
	observedAt := time.Now().UTC()

	records := make([]model.AssetRecord, 0, len(containers)+len(images))
	relationships := make([]model.RelationshipRecord, 0)

	imageRecordIDs := make(map[string]string, len(images))
	for _, image := range images {
		recordID := coremodule.StableRecordID("container_image", hostID, "docker", image.ID)
		imageRecordIDs[image.ID] = recordID
		name := firstString(image.RepoTags)
		if name == "" {
			name = image.ID
		}
		records = append(records, model.AssetRecord{
			RecordID: recordID, RecordType: "container_image", HostID: hostID,
			ScopeID: hostID, ScopeType: "host", Name: name, Platform: providers.Platform(),
			States:        model.AssetStates{Installed: true, Loaded: true},
			FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact",
			Attributes: map[string]any{
				"image_id":     image.ID,
				"repo_tags":    image.RepoTags,
				"repo_digests": image.RepoDigests,
				"size_bytes":   image.SizeBytes,
				"created_at":   image.CreatedAt,
				"labels":       security.RedactLabels(image.Labels),
			},
			Evidence: []model.Evidence{{Provider: provider.CapabilityContainer, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
	}

	containerRecordIDs := make(map[string]string, len(containers))
	for _, item := range containers {
		recordID := coremodule.StableRecordID("container", hostID, "docker", item.ID)
		containerRecordIDs[item.ID] = recordID
		attributes := map[string]any{
			"container_id": item.ID, "runtime": "docker",
			"image_id": item.ImageID, "image_name": item.ImageName, "image_tag": item.ImageTag,
			"state": item.State, "status": item.Status,
			"ports": item.Ports, "mounts": item.Mounts, "labels": security.RedactLabels(item.Labels),
		}
		if portAvailable {
			attributes["listening_host_ports"] = listeningPublishedPorts(item.Ports, listeningPorts)
		}
		records = append(records, model.AssetRecord{
			RecordID: recordID, RecordType: "container", HostID: hostID,
			ScopeID: hostID, ScopeType: "host", Name: item.Name, Platform: providers.Platform(),
			States:        model.AssetStates{Loaded: true, Running: item.State == "running", Exposed: len(item.Ports) > 0},
			FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact",
			Attributes: attributes,
			Evidence:    []model.Evidence{{Provider: provider.CapabilityContainer, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
		if imageID := imageRecordIDs[item.ImageID]; imageID != "" {
			relationships = append(relationships, model.RelationshipRecord{
				RecordID:         coremodule.StableRecordID("relationship", "based_on", recordID, imageID),
				RelationshipType: "based_on", FromID: recordID, ToID: imageID,
				ObservedAt: observedAt, Confidence: "exact",
				Evidence: []model.Evidence{{Provider: provider.CapabilityContainer, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
			})
		}
		relationships = append(relationships, model.RelationshipRecord{
			RecordID:         coremodule.StableRecordID("relationship", "runs_on", recordID, hostID),
			RelationshipType: "runs_on", FromID: recordID, ToID: hostID,
			ObservedAt: observedAt, Confidence: "exact",
			Evidence: []model.Evidence{{Provider: provider.CapabilityContainer, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
	}

	seen := make(map[string]bool)
	for _, process := range processes {
		containerID, ok := matchingContainerID(process.Cgroups, containerRecordIDs)
		if !ok {
			continue
		}
		processID := pidRecordIDs[process.PID]
		if processID == "" {
			continue
		}
		key := processID + "\x00" + containerRecordIDs[containerID]
		if seen[key] {
			continue
		}
		seen[key] = true
		relationships = append(relationships, model.RelationshipRecord{
			RecordID:         coremodule.StableRecordID("relationship", "runs_in", processID, containerRecordIDs[containerID]),
			RelationshipType: "runs_in", FromID: processID, ToID: containerRecordIDs[containerID],
			ObservedAt: observedAt, Confidence: "exact",
			Evidence: []model.Evidence{{Provider: provider.CapabilityContainer, SourceType: "dependency", ObservedAt: observedAt, Confidence: "exact"}},
		})
	}

	data := coremodule.NewModuleResult("container", status, scope(hostID), records, relationships)
	return coremodule.Result{Data: data, Internal: containers}
}

// matchingContainerID 返回 cgroup 路径中出现的容器 ID（完整 64 位十六进制）。
// 按 / 分段精确匹配，避免全量子串扫描可能带来的误匹配。
func matchingContainerID(cgroups []string, containerIDs map[string]string) (string, bool) {
	for _, cgroup := range cgroups {
		for _, token := range strings.Split(cgroup, "/") {
			id := strings.TrimPrefix(token, "docker-")
			id = strings.TrimSuffix(id, ".scope")
			if _, ok := containerIDs[id]; ok {
				return id, true
			}
		}
	}
	return "", false
}

// listeningHostPorts 返回 port 软依赖里正在监听的宿主端口集合，以及该软依赖是否可用。
func listeningHostPorts(request coremodule.Request) (map[int]bool, bool) {
	dependency, ok := request.Dependencies["port"]
	if !ok {
		return nil, false
	}
	ports := make(map[int]bool)
	for _, record := range dependency.Data.Records {
		if record.RecordType != "port" {
			continue
		}
		if port, ok := record.Attributes["local_port"].(int); ok {
			ports[port] = true
		}
	}
	return ports, true
}

// listeningPublishedPorts 返回已发布且宿主确实在监听的宿主端口列表。
func listeningPublishedPorts(containerPorts []model.ContainerPort, listening map[int]bool) []int {
	result := make([]int, 0)
	for _, port := range containerPorts {
		if port.PublicPort > 0 && listening[port.PublicPort] {
			result = append(result, port.PublicPort)
		}
	}
	return result
}

func hostDependency(request coremodule.Request) (model.Host, bool) {
	dependency, ok := request.Dependencies["host"]
	if !ok {
		return model.Host{}, false
	}
	host, ok := dependency.Internal.(model.Host)
	return host, ok
}

func processFacts(request coremodule.Request) []model.Process {
	dependency, ok := request.Dependencies["process"]
	if !ok {
		return nil
	}
	processes, _ := dependency.Internal.([]model.Process)
	return processes
}

func processPIDRecordIDs(request coremodule.Request) map[int]string {
	result := make(map[int]string)
	dependency, ok := request.Dependencies["process"]
	if !ok {
		return result
	}
	for _, record := range dependency.Data.Records {
		if record.RecordType != "process" {
			continue
		}
		if pid, ok := record.Attributes["pid"].(int); ok {
			result[pid] = record.RecordID
		}
	}
	return result
}

func firstString(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func scope(hostID string) []string {
	if hostID == "" {
		return []string{"host"}
	}
	return []string{hostID}
}

func failedResult(started time.Time, status model.Status, message string) coremodule.Result {
	collectorStatus := model.CollectorStatus{
		Collector: "container", Status: status, StartedAt: started,
		FinishedAt: time.Now().UTC(), Errors: []string{message},
	}
	return coremodule.Result{Data: coremodule.NewModuleResult("container", collectorStatus, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
}
