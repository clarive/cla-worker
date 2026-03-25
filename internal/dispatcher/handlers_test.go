package dispatcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/clarive/cla-worker-go/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPublisher struct {
	mu       sync.Mutex
	events   []string
	payloads []map[string]interface{}
	pushData []byte
	popData  []byte
	popErr   error
	pushErr  error
	closed   bool
}

func (m *mockPublisher) Publish(ctx context.Context, event string, data map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["_event"] = event
	m.payloads = append(m.payloads, payload)
	return nil
}

func (m *mockPublisher) Push(ctx context.Context, key, filename string, r io.Reader) error {
	if m.pushErr != nil {
		return m.pushErr
	}
	data, _ := io.ReadAll(r)
	m.mu.Lock()
	m.pushData = data
	m.mu.Unlock()
	return nil
}

func (m *mockPublisher) Pop(ctx context.Context, key string, w io.Writer) error {
	if m.popErr != nil {
		return m.popErr
	}
	if m.popData != nil {
		w.Write(m.popData)
	}
	return nil
}

func (m *mockPublisher) Close(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}

func (m *mockPublisher) getEvents() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockPublisher) getPayloads() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]map[string]interface{}, len(m.payloads))
	copy(result, m.payloads)
	return result
}

type mockExecutor struct {
	output string
	rc     int
	err    error
	delay  time.Duration
}

func (m *mockExecutor) Execute(ctx context.Context, cmd interface{}, chdir string) (string, int, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.err != nil {
		return "", 1, m.err
	}
	if m.output != "" {
		return m.output, m.rc, nil
	}
	return "mock output", 0, nil
}

type mockFS struct {
	writeErr error
	readData string
	readErr  error
	exists   bool
	isDir    bool
}

func (m *mockFS) WriteFileAtomic(filepath string, r io.Reader) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	io.ReadAll(r)
	return nil
}

func (m *mockFS) ReadFile(filepath string) (*os.File, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	tmp, _ := os.CreateTemp("", "mock-read-*")
	tmp.WriteString(m.readData)
	tmp.Seek(0, 0)
	return tmp, nil
}

func (m *mockFS) Exists(path string) (bool, bool, error) {
	return m.exists, m.isDir, nil
}

type mockEval struct {
	output string
	errStr string
	ret    interface{}
	err    error
}

func (m *mockEval) Eval(ctx context.Context, code string, stash map[string]interface{}) (*EvalResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &EvalResult{
		Output: m.output,
		Error:  m.errStr,
		Return: m.ret,
	}, nil
}

func runSingleMessage(t *testing.T, d *Dispatcher, event string, data map[string]interface{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch := make(chan pubsub.Message, 1)
	ch <- pubsub.Message{Event: event, OID: "oid-1", Data: data}
	close(ch)
	d.Run(ctx, ch)
}

func TestHandleExec_SimpleCommand(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockExecutor{output: "hello\n", rc: 0}
	d := New(mp, me, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{"cmd": "echo hello"})

	pubs := mp.getPayloads()
	var resultPayload map[string]interface{}
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			resultPayload = p
			break
		}
	}
	require.NotNil(t, resultPayload)
	assert.Equal(t, 0, resultPayload["rc"])
	assert.Equal(t, "hello\n", resultPayload["output"])
}

func TestHandleExec_ArrayCommand(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockExecutor{output: "array result", rc: 0}
	d := New(mp, me, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{
		"cmd": []interface{}{"echo", "test"},
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			assert.Equal(t, 0, p["rc"])
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandleExec_NonZeroExit(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockExecutor{output: "error output", rc: 42}
	d := New(mp, me, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{"cmd": "exit 42"})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			assert.Equal(t, 42, p["rc"])
		}
	}
}

func TestHandleExec_CommandNotFound(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockExecutor{err: errors.New("command not found")}
	d := New(mp, me, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{"cmd": "nonexistent"})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			assert.Equal(t, 99, p["rc"])
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandleExec_MissingCmd(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandleExec_WithChdir(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockExecutor{output: "/tmp\n", rc: 0}
	d := New(mp, me, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{
		"cmd":   "pwd",
		"chdir": "/tmp",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			assert.Equal(t, 0, p["rc"])
		}
	}
}

func TestHandleExec_LargeOutput(t *testing.T) {
	mp := &mockPublisher{}
	bigOutput := strings.Repeat("x", 1024*1024)
	me := &mockExecutor{output: bigOutput, rc: 0}
	d := New(mp, me, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{"cmd": "generate_output"})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			output, _ := p["output"].(string)
			assert.Equal(t, len(bigOutput), len(output))
		}
	}
}

func TestHandlePutFile_Success(t *testing.T) {
	mp := &mockPublisher{popData: []byte("file data")}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.put_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/tmp/test.txt",
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.put_file.done")
}

func TestHandlePutFile_MissingFilekey(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.put_file", map[string]interface{}{
		"filepath": "/tmp/test.txt",
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			out, _ := p["output"].(string)
			assert.Contains(t, out, "Missing filekey")
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandlePutFile_MissingFilepath(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.put_file", map[string]interface{}{
		"filekey": "fk1",
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			out, _ := p["output"].(string)
			assert.Contains(t, out, "Missing filepath")
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandlePutFile_PopError(t *testing.T) {
	mp := &mockPublisher{popErr: errors.New("network error")}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.put_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/tmp/test.txt",
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.put_file.fail")
}

func TestHandlePutFile_WriteError(t *testing.T) {
	mp := &mockPublisher{popData: []byte("data")}
	mfs := &mockFS{writeErr: errors.New("disk full")}
	d := New(mp, &mockExecutor{}, mfs, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.put_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/tmp/test.txt",
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.put_file.fail")
}

func TestHandleGetFile_Success(t *testing.T) {
	mp := &mockPublisher{}
	mfs := &mockFS{readData: "file content"}
	d := New(mp, &mockExecutor{}, mfs, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.get_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/tmp/test.txt",
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.get_file.done")
	assert.NotNil(t, mp.pushData)
}

func TestHandleGetFile_MissingFilekey(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.get_file", map[string]interface{}{
		"filepath": "/tmp/test.txt",
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandleGetFile_MissingFilepath(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.get_file", map[string]interface{}{
		"filekey": "fk1",
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandleGetFile_FileNotFound(t *testing.T) {
	mp := &mockPublisher{}
	mfs := &mockFS{readErr: errors.New("file not found")}
	d := New(mp, &mockExecutor{}, mfs, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.get_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/nonexistent.txt",
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.get_file.fail")
}

func TestHandleGetFile_PushError(t *testing.T) {
	mp := &mockPublisher{pushErr: errors.New("push failed")}
	mfs := &mockFS{readData: "data"}
	d := New(mp, &mockExecutor{}, mfs, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.get_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/tmp/test.txt",
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.get_file.fail")
}

func TestHandleCapable_AllMatch(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, []string{"linux", "docker"}, "w1", nil, nil)

	runSingleMessage(t, d, "worker.capable", map[string]interface{}{
		"tags": []interface{}{"linux", "docker"},
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.capable.reply")
}

func TestHandleCapable_PartialMatch(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, []string{"linux"}, "w1", nil, nil)

	runSingleMessage(t, d, "worker.capable", map[string]interface{}{
		"tags": []interface{}{"linux", "docker"},
	})

	events := mp.getEvents()
	hasReply := false
	for _, e := range events {
		if e == "worker.capable.reply" {
			hasReply = true
		}
	}
	assert.False(t, hasReply)
}

func TestHandleCapable_SubsetMatch(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, []string{"linux", "docker", "go"}, "w1", nil, nil)

	runSingleMessage(t, d, "worker.capable", map[string]interface{}{
		"tags": []interface{}{"linux"},
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.capable.reply")
}

func TestHandleCapable_EmptyTags(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, []string{"linux"}, "w1", nil, nil)

	runSingleMessage(t, d, "worker.capable", map[string]interface{}{
		"tags": []interface{}{},
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.capable.reply")
}

func TestHandleFileExists_Exists(t *testing.T) {
	mp := &mockPublisher{}
	mfs := &mockFS{exists: true, isDir: false}
	d := New(mp, &mockExecutor{}, mfs, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.file_exists", map[string]interface{}{
		"path": "/tmp/exists.txt",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.file_exists.reply" {
			assert.Equal(t, 1, p["exists"])
		}
	}
}

func TestHandleFileExists_NotExists(t *testing.T) {
	mp := &mockPublisher{}
	mfs := &mockFS{exists: false}
	d := New(mp, &mockExecutor{}, mfs, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.file_exists", map[string]interface{}{
		"path": "/nonexistent.txt",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.file_exists.reply" {
			assert.Equal(t, 0, p["exists"])
		}
	}
}

func TestHandleFileExists_Directory(t *testing.T) {
	mp := &mockPublisher{}
	mfs := &mockFS{exists: true, isDir: true}
	d := New(mp, &mockExecutor{}, mfs, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.file_exists", map[string]interface{}{
		"path": "/tmp",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.file_exists.reply" {
			assert.Equal(t, 1, p["exists"])
		}
	}
}

func TestHandleEval_SimpleExpression(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockEval{ret: 42}
	d := New(mp, &mockExecutor{}, &mockFS{}, me, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{
		"code": "1+1",
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.eval.done")
}

func TestHandleEval_ConsoleLog(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockEval{output: "logged text", ret: nil}
	d := New(mp, &mockExecutor{}, &mockFS{}, me, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{
		"code": "console.log('test')",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.eval.done" {
			assert.Equal(t, "logged text", p["output"])
		}
	}
}

func TestHandleEval_StashVars(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockEval{ret: 30}
	d := New(mp, &mockExecutor{}, &mockFS{}, me, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{
		"code":  "x + y",
		"stash": map[string]interface{}{"x": 10, "y": 20},
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.eval.done")
}

func TestHandleEval_SyntaxError(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockEval{errStr: "SyntaxError: unexpected token"}
	d := New(mp, &mockExecutor{}, &mockFS{}, me, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{
		"code": "function(",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.eval.done" {
			assert.NotEmpty(t, p["error"])
		}
	}
}

func TestHandleEval_RuntimeError(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockEval{errStr: "ReferenceError: x is not defined"}
	d := New(mp, &mockExecutor{}, &mockFS{}, me, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{
		"code": "x.y",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.eval.done" {
			assert.NotEmpty(t, p["error"])
		}
	}
}

func TestHandleEval_Timeout(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockEval{errStr: "timeout: execution timeout"}
	d := New(mp, &mockExecutor{}, &mockFS{}, me, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{
		"code": "while(true){}",
	})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.eval.done" {
			errStr, _ := p["error"].(string)
			assert.Contains(t, errStr, "timeout")
		}
	}
}

func TestHandleEval_EvaluatorError(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockEval{err: errors.New("evaluator broken")}
	d := New(mp, &mockExecutor{}, &mockFS{}, me, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{
		"code": "1+1",
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandleShutdown_CancelsContext(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	cancelled := false
	d.SetCancelFunc(func() { cancelled = true })

	runSingleMessage(t, d, "worker.shutdown", map[string]interface{}{
		"reason": "server request",
	})

	assert.True(t, cancelled)
	assert.Equal(t, 10, d.ShutdownCode())
	assert.True(t, mp.closed)
}

func TestPublishError_Format(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	ctx := context.Background()
	d.publishError(ctx, "oid-1", "worker.exec", "something failed")

	pubs := mp.getPayloads()
	require.Len(t, pubs, 1)
	assert.Equal(t, 99, pubs[0]["rc"])
	assert.Contains(t, pubs[0]["output"].(string), "worker.exec")
	assert.Contains(t, pubs[0]["output"].(string), "something failed")
}

// claude: verb filtering tests

func TestVerbDenied_ExecBlocked(t *testing.T) {
	mp := &mockPublisher{}
	// claude: only allow get_file, so exec should be denied
	allowed := map[string]bool{"get_file": true}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{"cmd": "echo foo"})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			out, _ := p["output"].(string)
			assert.Contains(t, out, "not allowed")
			found = true
		}
	}
	assert.True(t, found, "denied verb should produce rc=99 error")

	// claude: should also publish done
	events := mp.getEvents()
	assert.Contains(t, events, "worker.done")
}

func TestVerbDenied_GetFileBlocked(t *testing.T) {
	mp := &mockPublisher{}
	allowed := map[string]bool{"exec": true}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.get_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/tmp/foo.txt",
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			out, _ := p["output"].(string)
			assert.Contains(t, out, "not allowed")
			found = true
		}
	}
	assert.True(t, found)
}

func TestVerbDenied_PutFileBlocked(t *testing.T) {
	mp := &mockPublisher{}
	allowed := map[string]bool{"exec": true}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.put_file", map[string]interface{}{
		"filekey":  "fk1",
		"filepath": "/tmp/foo.txt",
	})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			out, _ := p["output"].(string)
			assert.Contains(t, out, "not allowed")
			found = true
		}
	}
	assert.True(t, found)
}

func TestVerbDenied_EvalBlocked(t *testing.T) {
	mp := &mockPublisher{}
	allowed := map[string]bool{"exec": true}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.eval", map[string]interface{}{"code": "1+1"})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			out, _ := p["output"].(string)
			assert.Contains(t, out, "not allowed")
			found = true
		}
	}
	assert.True(t, found)
}

func TestVerbDenied_FileExistsBlocked(t *testing.T) {
	mp := &mockPublisher{}
	allowed := map[string]bool{"exec": true}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.file_exists", map[string]interface{}{"path": "/tmp"})

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			out, _ := p["output"].(string)
			assert.Contains(t, out, "not allowed")
			found = true
		}
	}
	assert.True(t, found)
}

func TestVerbAllowed_NilMeansAllAllowed(t *testing.T) {
	// claude: nil allowedVerbs means no restrictions
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{output: "ok", rc: 0}, &mockFS{}, &mockEval{}, nil, "w1", nil, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{"cmd": "echo ok"})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			assert.Equal(t, 0, p["rc"])
			assert.Equal(t, "ok", p["output"])
		}
	}
}

func TestVerbAllowed_ExplicitlyAllowed(t *testing.T) {
	mp := &mockPublisher{}
	allowed := map[string]bool{"exec": true, "get_file": true}
	d := New(mp, &mockExecutor{output: "ok", rc: 0}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.exec", map[string]interface{}{"cmd": "echo ok"})

	pubs := mp.getPayloads()
	for _, p := range pubs {
		if p["_event"] == "worker.result" {
			assert.Equal(t, 0, p["rc"])
		}
	}
}

func TestVerbDenied_CapableAlwaysAllowed(t *testing.T) {
	// claude: worker.capable should always work regardless of verb restrictions
	mp := &mockPublisher{}
	allowed := map[string]bool{"exec": true} // no capable listed, but it's not controlled
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, []string{"linux"}, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.capable", map[string]interface{}{
		"tags": []interface{}{"linux"},
	})

	events := mp.getEvents()
	assert.Contains(t, events, "worker.capable.reply")
}

func TestVerbDenied_ShutdownAlwaysAllowed(t *testing.T) {
	// claude: worker.shutdown should always work regardless of verb restrictions
	mp := &mockPublisher{}
	allowed := map[string]bool{} // nothing allowed
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	cancelled := false
	d.SetCancelFunc(func() { cancelled = true })

	runSingleMessage(t, d, "worker.shutdown", map[string]interface{}{"reason": "test"})

	assert.True(t, cancelled)
	assert.Equal(t, 10, d.ShutdownCode())
}

func TestVerbDenied_ReadyAlwaysAllowed(t *testing.T) {
	// claude: worker.ready should always work regardless of verb restrictions
	mp := &mockPublisher{}
	allowed := map[string]bool{} // nothing allowed
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)

	runSingleMessage(t, d, "worker.ready", map[string]interface{}{})

	// ready produces only an ack, no error
	pubs := mp.getPayloads()
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok {
			assert.NotEqual(t, 99, rc, "worker.ready should not produce error")
		}
	}
}

func TestVerbDenied_AllDenied(t *testing.T) {
	// claude: empty allowed map denies all controlled verbs
	allowed := map[string]bool{}

	verbs := []struct {
		event string
		data  map[string]interface{}
	}{
		{"worker.exec", map[string]interface{}{"cmd": "echo foo"}},
		{"worker.eval", map[string]interface{}{"code": "1+1"}},
		{"worker.get_file", map[string]interface{}{"filekey": "fk", "filepath": "/tmp/x"}},
		{"worker.put_file", map[string]interface{}{"filekey": "fk", "filepath": "/tmp/x"}},
		{"worker.file_exists", map[string]interface{}{"path": "/tmp"}},
	}

	for _, v := range verbs {
		t.Run(v.event, func(t *testing.T) {
			mp := &mockPublisher{}
			d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", allowed, nil)
			runSingleMessage(t, d, v.event, v.data)

			pubs := mp.getPayloads()
			found := false
			for _, p := range pubs {
				if rc, ok := p["rc"]; ok && rc == 99 {
					out, _ := p["output"].(string)
					assert.Contains(t, out, "not allowed")
					found = true
				}
			}
			assert.True(t, found, "%s should be denied", v.event)
		})
	}
}

// -- Dummy unused import suppression --
var _ = bytes.NewBuffer
var _ = require.NoError
