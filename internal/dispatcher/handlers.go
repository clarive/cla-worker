package dispatcher

import (
	"bytes"
	"context"
	"fmt"
)

func (d *Dispatcher) handleReady(ctx context.Context, msgID string, data map[string]interface{}) {
}

func (d *Dispatcher) handleExec(ctx context.Context, msgID string, data map[string]interface{}) {
	cmd, ok := data["cmd"]
	if !ok {
		d.publishError(ctx, msgID, "worker.exec", "missing cmd field")
		d.publishDone(ctx, msgID)
		return
	}

	chdir, _ := data["chdir"].(string)

	// claude: create a cancellable child context so cancel_repl can kill this command
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	d.execMu.Lock()
	d.execCancel = cancel
	d.execMu.Unlock()

	output, rc, err := d.executor.Execute(execCtx, cmd, chdir)

	// claude: clear the stored cancel func now that exec is done
	d.execMu.Lock()
	d.execCancel = nil
	d.execMu.Unlock()

	if err != nil {
		if execCtx.Err() == context.Canceled {
			d.publishResult(ctx, msgID, map[string]interface{}{
				"ret":    "",
				"rc":     130,
				"output": "command cancelled by user",
			})
			d.publishDone(ctx, msgID)
			return
		}
		d.publishError(ctx, msgID, "worker.exec", err.Error())
		d.publishDone(ctx, msgID)
		return
	}

	d.publishResult(ctx, msgID, map[string]interface{}{
		"ret":    "",
		"rc":     rc,
		"output": output,
	})
	d.publishDone(ctx, msgID)
}

func (d *Dispatcher) handleExecCancel(ctx context.Context, msgID string, data map[string]interface{}) {
	d.execMu.Lock()
	cancel := d.execCancel
	d.execMu.Unlock()

	if cancel != nil {
		d.logger.Info("cancelling running exec command")
		cancel()
	} else {
		d.logger.Info("cancel requested but no exec command is running")
	}
}

func (d *Dispatcher) handleEval(ctx context.Context, msgID string, data map[string]interface{}) {
	code, _ := data["code"].(string)
	stash, _ := data["stash"].(map[string]interface{})

	result, err := d.evaluator.Eval(ctx, code, stash)
	if err != nil {
		d.publishError(ctx, msgID, "worker.eval", err.Error())
		return
	}

	payload := map[string]interface{}{
		"oid":    msgID,
		"output": result.Output,
	}
	if result.Error != "" {
		payload["error"] = result.Error
	}
	if result.Return != nil {
		payload["ret"] = result.Return
	}

	d.publisher.Publish(ctx, "worker.eval.done", payload)
}

func (d *Dispatcher) handlePutFile(ctx context.Context, msgID string, data map[string]interface{}) {
	filekey, _ := data["filekey"].(string)
	filepath, _ := data["filepath"].(string)

	if filekey == "" {
		d.publishError(ctx, msgID, "worker.put_file", "Missing filekey in put_file operation")
		d.publishDone(ctx, msgID)
		return
	}

	if filepath == "" {
		d.publishError(ctx, msgID, "worker.put_file", "Missing filepath in put_file operation")
		d.publishDone(ctx, msgID)
		return
	}

	var buf bytes.Buffer
	if err := d.publisher.Pop(ctx, filekey, &buf); err != nil {
		d.publisher.Publish(ctx, "worker.put_file.fail", map[string]interface{}{
			"oid":      msgID,
			"filekey":  filekey,
			"filepath": filepath,
			"error":    fmt.Sprintf("pop failed: %s", err),
		})
		return
	}

	if err := d.filesystem.WriteFileAtomic(filepath, &buf); err != nil {
		d.publisher.Publish(ctx, "worker.put_file.fail", map[string]interface{}{
			"oid":      msgID,
			"filekey":  filekey,
			"filepath": filepath,
			"error":    fmt.Sprintf("write failed: %s", err),
		})
		return
	}

	d.publisher.Publish(ctx, "worker.put_file.done", map[string]interface{}{
		"oid":      msgID,
		"filekey":  filekey,
		"filepath": filepath,
	})
	d.logger.Info("put_file done", "filepath", filepath)
}

func (d *Dispatcher) handleGetFile(ctx context.Context, msgID string, data map[string]interface{}) {
	filekey, _ := data["filekey"].(string)
	filepath, _ := data["filepath"].(string)

	if filekey == "" {
		d.publishError(ctx, msgID, "worker.get_file", "Missing filekey in get_file operation")
		d.publishDone(ctx, msgID)
		return
	}

	if filepath == "" {
		d.publishError(ctx, msgID, "worker.get_file", "Missing filepath in get_file operation")
		d.publishDone(ctx, msgID)
		return
	}

	f, err := d.filesystem.ReadFile(filepath)
	if err != nil {
		d.publisher.Publish(ctx, "worker.get_file.fail", map[string]interface{}{
			"oid":      msgID,
			"filekey":  filekey,
			"filepath": filepath,
			"error":    fmt.Sprintf("could not read file %s: %s", filepath, err),
		})
		return
	}
	defer f.Close()

	if err := d.publisher.Push(ctx, filekey, filepath, f); err != nil {
		d.publisher.Publish(ctx, "worker.get_file.fail", map[string]interface{}{
			"oid":      msgID,
			"filekey":  filekey,
			"filepath": filepath,
			"error":    err.Error(),
		})
		return
	}

	d.publisher.Publish(ctx, "worker.get_file.done", map[string]interface{}{
		"oid":      msgID,
		"filekey":  filekey,
		"filepath": filepath,
	})
	d.logger.Info("get_file done", "filepath", filepath)
}

func (d *Dispatcher) handleCapable(ctx context.Context, msgID string, data map[string]interface{}) {
	tagsRaw, _ := data["tags"].([]interface{})
	requestedTags := make([]string, len(tagsRaw))
	for i, t := range tagsRaw {
		requestedTags[i] = fmt.Sprintf("%v", t)
	}

	myTags := d.tags

	allMatch := true
	for _, rt := range requestedTags {
		found := false
		for _, mt := range myTags {
			if rt == mt {
				found = true
				break
			}
		}
		if !found {
			allMatch = false
			break
		}
	}

	if allMatch {
		d.publisher.Publish(ctx, "worker.capable.reply", map[string]interface{}{
			"oid":      msgID,
			"workerid": d.workerID,
			"tags":     myTags,
		})
	}
}

func (d *Dispatcher) handleFileExists(ctx context.Context, msgID string, data map[string]interface{}) {
	path, _ := data["path"].(string)

	exists, _, _ := d.filesystem.Exists(path)

	existsVal := 0
	if exists {
		existsVal = 1
	}

	d.publisher.Publish(ctx, "worker.file_exists.reply", map[string]interface{}{
		"oid":      msgID,
		"workerid": d.workerID,
		"exists":   existsVal,
	})
}

func (d *Dispatcher) handleShutdown(ctx context.Context, msgID string, data map[string]interface{}) {
	reason, _ := data["reason"].(string)
	d.logger.Warn("shutdown event received from server", "reason", reason)
	d.logger.Warn("trying to stop gracefully...")

	d.publisher.Close(ctx)
	d.logger.Info("worker shutdown on server request completed")

	if d.cancelFunc != nil {
		d.cancelFunc()
	}
	d.shutdownCode = 10
}

func (d *Dispatcher) publishResult(ctx context.Context, msgID string, result map[string]interface{}) {
	result["oid"] = msgID
	d.publisher.Publish(ctx, "worker.result", result)
}

func (d *Dispatcher) publishDone(ctx context.Context, msgID string) {
	d.publisher.Publish(ctx, "worker.done", map[string]interface{}{
		"oid": msgID,
	})
}

func (d *Dispatcher) publishError(ctx context.Context, msgID, cmd, errMsg string) {
	msg := fmt.Sprintf("during command %s: %s", cmd, errMsg)
	d.logger.Error(msg)
	d.publishResult(ctx, msgID, map[string]interface{}{
		"ret":    msg,
		"rc":     99,
		"output": msg,
	})
}
