package jseval

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEval_SimpleExpression(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "1 + 2", nil)
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Equal(t, int64(3), result.Return)
}

func TestEval_StringExpression(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "'hello' + ' ' + 'world'", nil)
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Equal(t, "hello world", result.Return)
}

func TestEval_ConsoleLog(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "console.log('test output'); 42", nil)
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Equal(t, "test output", result.Output)
	assert.Equal(t, int64(42), result.Return)
}

func TestEval_ConsoleLogMultipleArgs(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "console.log('a', 'b', 'c')", nil)
	require.NoError(t, err)
	assert.Equal(t, "a b c", result.Output)
}

func TestEval_StashVars(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	stash := map[string]interface{}{
		"x": 10,
		"y": 20,
	}
	result, err := e.Eval(context.Background(), "x + y", stash)
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Equal(t, int64(30), result.Return)
}

func TestEval_SyntaxError(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "function(", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

func TestEval_RuntimeError(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "undefined_var.property", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

func TestEval_Timeout(t *testing.T) {
	e := NewEvaluator(100*time.Millisecond, nil)
	result, err := e.Eval(context.Background(), "while(true) {}", nil)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "timeout")
}

func TestEval_UndefinedReturn(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "var x = 1;", nil)
	require.NoError(t, err)
	assert.Nil(t, result.Return)
}

func TestEval_NullReturn(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "null", nil)
	require.NoError(t, err)
	assert.Nil(t, result.Return)
}

func TestEval_ObjectReturn(t *testing.T) {
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(context.Background(), "({a: 1, b: 'hello'})", nil)
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	m, ok := result.Return.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, int64(1), m["a"])
	assert.Equal(t, "hello", m["b"])
}

func TestEval_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := NewEvaluator(5*time.Second, nil)
	result, err := e.Eval(ctx, "while(true){}", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}
