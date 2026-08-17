package container

import (
	"context"
	"errors"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

type fakeSource struct {
	containers    []model.Container
	images        []model.ContainerImage
	containersErr error
	imagesErr     error
}

func (source fakeSource) ListContainers(context.Context) ([]model.Container, error) {
	return source.containers, source.containersErr
}

func (source fakeSource) ListImages(context.Context) ([]model.ContainerImage, error) {
	return source.images, source.imagesErr
}

func TestCollectReturnsContainersAndImages(t *testing.T) {
	t.Parallel()

	source := fakeSource{
		containers: []model.Container{{ID: "abc", Name: "nginx", State: "running"}},
		images:     []model.ContainerImage{{ID: "img", RepoTags: []string{"nginx:latest"}}},
	}
	containers, images, status := Collect(context.Background(), source)
	if status.Status != model.StatusOK {
		t.Fatalf("status = %+v", status)
	}
	if len(containers) != 1 || len(images) != 1 {
		t.Fatalf("containers=%d images=%d", len(containers), len(images))
	}
}

func TestCollectUnsupportedWhenRuntimeUnavailable(t *testing.T) {
	t.Parallel()

	_, _, status := Collect(context.Background(), fakeSource{containersErr: ErrRuntimeUnavailable})
	if status.Status != model.StatusUnsupported {
		t.Fatalf("status = %+v, want unsupported", status)
	}
}

func TestCollectFailedWhenRuntimeQueryFails(t *testing.T) {
	t.Parallel()

	_, _, status := Collect(context.Background(), fakeSource{containersErr: errors.New("permission denied")})
	if status.Status != model.StatusFailed {
		t.Fatalf("status = %+v, want failed", status)
	}
}

func TestCollectPartialWhenImagesFail(t *testing.T) {
	t.Parallel()

	source := fakeSource{
		containers: []model.Container{{ID: "abc", Name: "nginx"}},
		imagesErr:  errors.New("image query failed"),
	}
	containers, images, status := Collect(context.Background(), source)
	if status.Status != model.StatusPartial {
		t.Fatalf("status = %+v, want partial", status)
	}
	if len(containers) != 1 || len(images) != 0 {
		t.Fatalf("containers=%d images=%d", len(containers), len(images))
	}
}

func TestSplitImageRef(t *testing.T) {
	t.Parallel()

	tests := []struct{ ref, wantName, wantTag string }{
		{"nginx:latest", "nginx", "latest"},
		{"nginx", "nginx", ""},
		{"registry:5000/team/nginx:1.2", "registry:5000/team/nginx", "1.2"},
	}
	for _, test := range tests {
		name, tag := splitImageRef(test.ref)
		if name != test.wantName || tag != test.wantTag {
			t.Fatalf("splitImageRef(%q) = %q,%q want %q,%q", test.ref, name, tag, test.wantName, test.wantTag)
		}
	}
}
