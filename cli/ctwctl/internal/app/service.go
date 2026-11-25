package app

import (
	"context"
	"errors"
	"regexp"
)

var (
	ErrInvalidName = errors.New("invalid Deployment name")
	deploymentName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)
)

type Client interface {
	ListApps(ctx context.Context) ([]Status, error)
	RestartApp(ctx context.Context, name string) (string, error)
}

type Apps struct {
	client Client
}

func NewApps(client Client) *Apps {
	return &Apps{client: client}
}

func (a *Apps) List(ctx context.Context) ([]Status, error) {
	return a.client.ListApps(ctx)
}

func (a *Apps) Restart(ctx context.Context, name string) (string, error) {
	if !deploymentName.MatchString(name) {
		return "", ErrInvalidName
	}
	return a.client.RestartApp(ctx, name)
}
