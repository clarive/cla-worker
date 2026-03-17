package jseval

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dop251/goja"
)

type EvalResult struct {
	Output string
	Error  string
	Return interface{}
}

type Evaluator struct {
	timeout time.Duration
	logger  *slog.Logger
}

func NewEvaluator(timeout time.Duration, logger *slog.Logger) *Evaluator {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Evaluator{timeout: timeout, logger: logger}
}

func (e *Evaluator) Eval(ctx context.Context, code string, stash map[string]interface{}) (*EvalResult, error) {
	result := &EvalResult{}
	var output strings.Builder

	vm := goja.New()

	consoleObj := vm.NewObject()
	consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		for i, arg := range call.Arguments {
			if i > 0 {
				output.WriteString(" ")
			}
			output.WriteString(arg.String())
		}
		return goja.Undefined()
	})
	vm.Set("console", consoleObj)

	for k, v := range stash {
		vm.Set(k, v)
	}

	evalCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		select {
		case <-evalCtx.Done():
			vm.Interrupt("execution timeout")
		case <-done:
		}
	}()

	val, err := vm.RunString(code)
	close(done)

	result.Output = output.String()

	if err != nil {
		if jErr, ok := err.(*goja.InterruptedError); ok {
			result.Error = fmt.Sprintf("timeout: %s", jErr.Value())
		} else {
			result.Error = err.Error()
		}
		return result, nil
	}

	if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
		result.Return = val.Export()
	}

	return result, nil
}
