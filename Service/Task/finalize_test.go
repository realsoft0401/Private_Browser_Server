package Task

import (
	"errors"
	"testing"

	"private_browser_server/EdgeClient"
	model "private_browser_server/Models/Task"
)

func TestDecideFromEdgeTaskSuccess(t *testing.T) {
	decision := DecideFromEdgeTask(&EdgeClient.EdgeTask{Status: "success", Message: "ok"}, nil)
	if !decision.Final || decision.Status != model.TaskStatusSuccess {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestDecideFromEdgeTaskRunning(t *testing.T) {
	decision := DecideFromEdgeTask(&EdgeClient.EdgeTask{Status: "running", Message: "working"}, nil)
	if decision.Final || decision.Status != model.TaskStatusRunning {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestDecideFromEdgeTaskQueryErrorFails(t *testing.T) {
	decision := DecideFromEdgeTask(nil, errors.New("404 task not found"))
	if !decision.Final || decision.Status != model.TaskStatusFailed {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestDecideFromEdgeTaskUnknownFails(t *testing.T) {
	decision := DecideFromEdgeTask(&EdgeClient.EdgeTask{Status: "manual_check_required"}, nil)
	if !decision.Final || decision.Status != model.TaskStatusFailed {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}
