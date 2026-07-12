package handler

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPageHandlerDeadSurfaceStaysRemoved(t *testing.T) {
	_, err := os.Stat("page_handler.go")
	require.True(t, errors.Is(err, os.ErrNotExist), "SuperLLM 不应恢复 Sub2API 自定义页面 handler")
}
