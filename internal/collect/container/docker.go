package container

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// DockerSource 通过 Docker Engine API 的 Unix socket 查询容器与镜像。
type DockerSource struct {
	socketPath string
	client     *http.Client
}

// NewDockerSource 创建指向指定 Unix socket 的 Docker 源。
func NewDockerSource(socketPath string) *DockerSource {
	return &DockerSource{
		socketPath: socketPath,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
				},
			},
		},
	}
}

// ListContainers 返回全部容器（含已停止），socket 缺失时返回 ErrRuntimeUnavailable。
func (source *DockerSource) ListContainers(ctx context.Context) ([]model.Container, error) {
	if _, err := os.Stat(source.socketPath); err != nil {
		return nil, fmt.Errorf("%w: socket %s 不存在", ErrRuntimeUnavailable, source.socketPath)
	}
	var items []dockerContainer
	if err := source.get(ctx, "/containers/json?all=1", &items); err != nil {
		return nil, err
	}
	containers := make([]model.Container, 0, len(items))
	for _, item := range items {
		name, tag := splitImageRef(item.Image)
		containers = append(containers, model.Container{
			ID:        item.ID,
			Name:      strings.TrimPrefix(firstString(item.Names), "/"),
			ImageID:   item.ImageID,
			ImageName: name,
			ImageTag:  tag,
			State:     item.State,
			Status:    item.Status,
			Ports:     toContainerPorts(item.Ports),
			Mounts:    toContainerMounts(item.Mounts),
			Labels:    item.Labels,
		})
	}
	return containers, nil
}

// ListImages 返回全部镜像，socket 缺失时返回 ErrRuntimeUnavailable。
func (source *DockerSource) ListImages(ctx context.Context) ([]model.ContainerImage, error) {
	if _, err := os.Stat(source.socketPath); err != nil {
		return nil, fmt.Errorf("%w: socket %s 不存在", ErrRuntimeUnavailable, source.socketPath)
	}
	var items []dockerImage
	if err := source.get(ctx, "/images/json", &items); err != nil {
		return nil, err
	}
	images := make([]model.ContainerImage, 0, len(items))
	for _, item := range items {
		images = append(images, model.ContainerImage{
			ID:          item.ID,
			RepoTags:    item.RepoTags,
			RepoDigests: item.RepoDigests,
			SizeBytes:   item.Size,
			CreatedAt:   item.Created,
			Labels:      item.Labels,
		})
	}
	return images, nil
}

func (source *DockerSource) get(ctx context.Context, apiPath string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix"+apiPath, nil)
	if err != nil {
		return err
	}
	response, err := source.client.Do(request)
	if err != nil {
		return fmt.Errorf("docker api %s: %w", apiPath, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("docker api %s: status %s", apiPath, response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("docker api %s: %w", apiPath, err)
	}
	return nil
}

type dockerContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Ports   []dockerPort      `json:"Ports"`
	Mounts  []dockerMount     `json:"Mounts"`
	Labels  map[string]string `json:"Labels"`
}

type dockerPort struct {
	IP          string `json:"IP"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}

type dockerMount struct {
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
}

type dockerImage struct {
	ID          string            `json:"Id"`
	RepoTags    []string          `json:"RepoTags"`
	RepoDigests []string          `json:"RepoDigests"`
	Size        int64             `json:"Size"`
	Created     int64             `json:"Created"`
	Labels      map[string]string `json:"Labels"`
}

func firstString(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

// splitImageRef 把镜像引用拆分为名称与标签；无标签时标签为空。
func splitImageRef(ref string) (name, tag string) {
	slash := strings.LastIndex(ref, "/")
	colon := strings.LastIndex(ref, ":")
	if colon > slash {
		return ref[:colon], ref[colon+1:]
	}
	return ref, ""
}

func toContainerPorts(ports []dockerPort) []model.ContainerPort {
	result := make([]model.ContainerPort, 0, len(ports))
	for _, port := range ports {
		result = append(result, model.ContainerPort{
			IP:          port.IP,
			PrivatePort: port.PrivatePort,
			PublicPort:  port.PublicPort,
			Type:        port.Type,
		})
	}
	return result
}

func toContainerMounts(mounts []dockerMount) []model.ContainerMount {
	result := make([]model.ContainerMount, 0, len(mounts))
	for _, mount := range mounts {
		result = append(result, model.ContainerMount{
			Source:      mount.Source,
			Destination: mount.Destination,
			Mode:        mount.Mode,
		})
	}
	return result
}
