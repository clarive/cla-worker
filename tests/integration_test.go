//go:build integration

package tests

import (
	"fmt"
	"net/http"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/clarive/cla-worker-go/internal/config"
	"github.com/clarive/cla-worker-go/internal/dispatcher"
	"github.com/clarive/cla-worker-go/internal/executor"
	"github.com/clarive/cla-worker-go/internal/filetransfer"
	"github.com/clarive/cla-worker-go/internal/jseval"
	"github.com/clarive/cla-worker-go/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jsEvalAdapter struct {
	eval *jseval.Evaluator
}

func (a *jsEvalAdapter) Eval(ctx context.Context, code string, stash map[string]interface{}) (*dispatcher.EvalResult, error) {
	result, err := a.eval.Eval(ctx, code, stash)
	if err != nil {
		return nil, err
	}
	return &dispatcher.EvalResult{
		Output: result.Output,
		Error:  result.Error,
		Return: result.Return,
	}, nil
}

func setupIntegration(t *testing.T) (*MockClariveServer, *pubsub.Client) {
	t.Helper()
	ms := NewMockClariveServer()
	t.Cleanup(ms.Close)

	ps := pubsub.NewClient(
		pubsub.WithBaseURL(ms.Server.URL),
		pubsub.WithID("test-worker"),
		pubsub.WithToken("test-token"),
		pubsub.WithTags([]string{"linux", "test"}),
		pubsub.WithOrigin("test@host/1"),
	)

	return ms, ps
}

func TestIntegration_Register_Success(t *testing.T) {
	ms := NewMockClariveServer()
	defer ms.Close()

	ps := pubsub.NewClient(
		pubsub.WithBaseURL(ms.Server.URL),
		pubsub.WithID("w1"),
	)

	result, err := ps.Register(context.Background(), "valid-passkey")
	require.NoError(t, err)
	assert.Equal(t, "test-token-abc", result.Token)
	assert.Contains(t, result.Projects, "project-1")
}

func TestIntegration_Register_RejectedPasskey(t *testing.T) {
	ms := NewMockClariveServer()
	defer ms.Close()

	ms.RegisterHandler = func(r *http.Request) (int, interface{}) {
		return 403, map[string]interface{}{"error": "passkey rejected"}
	}

	ps := pubsub.NewClient(pubsub.WithBaseURL(ms.Server.URL), pubsub.WithID("w1"))
	_, err := ps.Register(context.Background(), "bad-passkey")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestIntegration_Register_InvalidPasskey(t *testing.T) {
	ms := NewMockClariveServer()
	defer ms.Close()

	ms.RegisterHandler = func(r *http.Request) (int, interface{}) {
		return 400, map[string]interface{}{"error": "invalid passkey format"}
	}

	ps := pubsub.NewClient(pubsub.WithBaseURL(ms.Server.URL), pubsub.WithID("w1"))
	_, err := ps.Register(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestIntegration_Register_ServerError(t *testing.T) {
	ms := NewMockClariveServer()
	defer ms.Close()

	ms.RegisterHandler = func(r *http.Request) (int, interface{}) {
		return 500, "internal error"
	}

	ps := pubsub.NewClient(pubsub.WithBaseURL(ms.Server.URL), pubsub.WithID("w1"))
	_, err := ps.Register(context.Background(), "key")
	require.Error(t, err)
}

func TestIntegration_Register_ServerDown(t *testing.T) {
	ps := pubsub.NewClient(pubsub.WithBaseURL("http://127.0.0.1:1"), pubsub.WithID("w1"))
	_, err := ps.Register(context.Background(), "key")
	require.Error(t, err)
}

func TestIntegration_Unregister(t *testing.T) {
	ms := NewMockClariveServer()
	defer ms.Close()

	ps := pubsub.NewClient(
		pubsub.WithBaseURL(ms.Server.URL),
		pubsub.WithID("w1"),
		pubsub.WithToken("tok"),
	)

	err := ps.Unregister(context.Background())
	require.NoError(t, err)
}

func TestIntegration_ConnectAndExec(t *testing.T) {
	ms, ps := setupIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-1", "worker.exec", map[string]interface{}{
			"cmd": "echo integration-test",
		})
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, []string{"linux", "test"}, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	go func() {
		time.Sleep(3 * time.Second)
		cancel()
	}()

	disp.Run(ctx, messages)

	pubs := ms.GetPublishedByEvent("worker.result")
	require.NotEmpty(t, pubs, "should have published a result")
	assert.Contains(t, pubs[0].Data["output"], "integration-test")
}

func TestIntegration_MultipleCommands(t *testing.T) {
	ms, ps := setupIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		for i := 0; i < 5; i++ {
			ms.SendCommand(
				fmt.Sprintf("oid-%d", i),
				"worker.exec",
				map[string]interface{}{"cmd": fmt.Sprintf("echo cmd-%d", i)},
			)
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, []string{"linux"}, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	results := ms.GetPublishedByEvent("worker.result")
	assert.GreaterOrEqual(t, len(results), 3)
}

func TestIntegration_PutFile(t *testing.T) {
	ms, ps := setupIntegration(t)
	ms.PopData = []byte("file content from server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.txt")

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-put", "worker.put_file", map[string]interface{}{
			"filekey":  "fk-1",
			"filepath": outFile,
		})
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, nil, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Equal(t, "file content from server", string(data))
}

func TestIntegration_GetFile(t *testing.T) {
	ms, ps := setupIntegration(t)

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(srcFile, []byte("local file data"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-get", "worker.get_file", map[string]interface{}{
			"filekey":  "fk-2",
			"filepath": srcFile,
		})
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, nil, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	pushed := <-ms.PushReceived
	assert.Equal(t, "local file data", string(pushed))

	doneEvents := ms.GetPublishedByEvent("worker.get_file.done")
	assert.NotEmpty(t, doneEvents)
}

func TestIntegration_Eval(t *testing.T) {
	ms, ps := setupIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-eval", "worker.eval", map[string]interface{}{
			"code": "1 + 2",
		})
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, nil, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	evalDone := ms.GetPublishedByEvent("worker.eval.done")
	require.NotEmpty(t, evalDone)
}

func TestIntegration_Capable(t *testing.T) {
	ms, ps := setupIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-cap", "worker.capable", map[string]interface{}{
			"tags": []interface{}{"linux"},
		})
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, []string{"linux", "test"}, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	replies := ms.GetPublishedByEvent("worker.capable.reply")
	require.NotEmpty(t, replies)
}

func TestIntegration_FileExists(t *testing.T) {
	ms, ps := setupIntegration(t)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "exists.txt")
	os.WriteFile(testFile, []byte("x"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-fe", "worker.file_exists", map[string]interface{}{
			"path": testFile,
		})
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, nil, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	replies := ms.GetPublishedByEvent("worker.file_exists.reply")
	require.NotEmpty(t, replies)
	assert.Equal(t, float64(1), replies[0].Data["exists"])
}

func TestIntegration_Shutdown(t *testing.T) {
	ms, ps := setupIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-sd", "worker.shutdown", map[string]interface{}{
			"reason": "test shutdown",
		})
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, nil, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	assert.Equal(t, 10, disp.ShutdownCode())
}

func TestIntegration_InvalidJSON(t *testing.T) {
	ms, ps := setupIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SSEEvents <- "data: {invalid json}\n\n"
		time.Sleep(100 * time.Millisecond)
		ms.SendCommand("oid-ok", "worker.exec", map[string]interface{}{
			"cmd": "echo recovered",
		})
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, nil, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	results := ms.GetPublishedByEvent("worker.result")
	require.NotEmpty(t, results)
}

func TestIntegration_UnknownCommand(t *testing.T) {
	ms, ps := setupIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages, err := ps.Connect(ctx)
	require.NoError(t, err)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ms.SendCommand("oid-unk", "worker.unknown_cmd", map[string]interface{}{})
		time.Sleep(2 * time.Second)
		cancel()
	}()

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(5*time.Second, nil)}

	disp := dispatcher.New(ps, exec, fs, eval, nil, "test-worker", nil)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)

	results := ms.GetPublishedByEvent("worker.result")
	require.NotEmpty(t, results)
	assert.Equal(t, float64(99), results[0].Data["rc"])
}

func TestIntegration_PushPop_CLI(t *testing.T) {
	ms := NewMockClariveServer()
	defer ms.Close()
	ms.PopData = []byte("popped data")

	ps := pubsub.NewClient(
		pubsub.WithBaseURL(ms.Server.URL),
		pubsub.WithID("w1"),
		pubsub.WithToken("tok"),
	)

	err := ps.Push(context.Background(), "test-key", "file.txt",
		bytes.NewReader([]byte("pushed data")))
	require.NoError(t, err)

	pushed := <-ms.PushReceived
	assert.Equal(t, "pushed data", string(pushed))

	var buf bytes.Buffer
	err = ps.Pop(context.Background(), "test-key", &buf)
	require.NoError(t, err)
	assert.Equal(t, "popped data", buf.String())
}

func TestIntegration_ConfigLoadYAML(t *testing.T) {
	cfg, err := config.Load("testdata/valid_config.yml")
	require.NoError(t, err)
	assert.Equal(t, "worker-1", cfg.ID)
	assert.Equal(t, "tok-abc123", cfg.Token)
}

func TestIntegration_ConfigLoadTOML(t *testing.T) {
	cfg, err := config.Load("testdata/valid_config.toml")
	require.NoError(t, err)
	assert.Equal(t, "worker-1", cfg.ID)
	assert.Equal(t, "tok-abc123", cfg.Token)
}

// --- suppress unused import warnings ---
var _ = fmt.Sprintf
