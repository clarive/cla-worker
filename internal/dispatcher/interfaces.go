package dispatcher

import (
	"context"
	"io"
	"os"
)

type Publisher interface {
	Publish(ctx context.Context, event string, data map[string]interface{}) error
	Push(ctx context.Context, key, filename string, r io.Reader) error
	Pop(ctx context.Context, key string, w io.Writer) error
	Close(ctx context.Context) error
	SetID(id string)
}

type CommandExecutor interface {
	Execute(ctx context.Context, cmd interface{}, chdir string) (output string, rc int, err error)
}

type FileSystem interface {
	WriteFileAtomic(filepath string, r io.Reader) error
	ReadFile(filepath string) (*os.File, error)
	Exists(path string) (exists bool, isDir bool, err error)
}

type JSEvaluator interface {
	Eval(ctx context.Context, code string, stash map[string]interface{}) (*EvalResult, error)
}

type EvalResult struct {
	Output string
	Error  string
	Return interface{}
}
