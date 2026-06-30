package Node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	NodeDAO "private_browser_server/Dao/Node"
	TaskDAO "private_browser_server/Dao/Task"
	TaskModel "private_browser_server/Models/Task"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
	EdgeClientService "private_browser_server/Service/EdgeClient"
	TaskService "private_browser_server/Service/Task"
)

type slotReconcileRequest struct {
	Source string `json:"source"`
}

func SlotReconcile(c *gin.Context) {
	clientID := strings.TrimSpace(c.Param("clientId"))
	if clientID == "" {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "clientId 不能为空")
		return
	}
	var request slotReconcileRequest
	if err := c.ShouldBindJSON(&request); err != nil && strings.TrimSpace(err.Error()) != "EOF" {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "slot-reconcile request body 非法")
		return
	}

	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	node, err := NodeRepo.NewRepository().GetByClientID(requestCtx, clientID)
	if err == NodeRepo.ErrNotFound {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, "edge client not found")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, err.Error())
		return
	}
	if node.HealthStatus != "healthy" || node.DiscoveryStatus != "verified" {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, "edge client is not healthy and verified")
		return
	}

	taskID, err := TaskService.GetService().CreateTask(requestCtx, &TaskDAO.Row{
		MainAccountID: node.MainAccountID,
		ClientID:      node.ClientID,
		TaskType:      "slot_reconcile",
		ResourceType:  "edge_client",
		ResourceID:    node.ClientID,
		Status:        TaskModel.StatusPending,
	})
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, err.Error())
		return
	}

	go runSlotReconcile(taskID, node.ClientID, strings.TrimSpace(node.BaseURL), node.TargetSlotCount, strings.TrimSpace(request.Source))

	HttpResponse.ResponseSuccess(c, gin.H{
		"taskId":       taskID,
		"taskType":     "slot_reconcile",
		"resourceType": "edge_client",
		"resourceId":   node.ClientID,
		"eventsUrl":    fmt.Sprintf("/api/v1/server-tasks/%s/events", taskID),
	})
}

func runSlotReconcile(taskID, clientID, baseURL string, targetSlotCount int64, source string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	publisher := TaskService.GetService()
	_ = publisher.PublishProgress(ctx, taskID, newSlotReconcileEvent(TaskModel.EventProgress, taskID, clientID, "", "load_client", TaskModel.StatusPending, "task accepted", "", ""))

	slots, err := EdgeClientService.New().ListSlots(ctx, baseURL)
	if err != nil {
		_ = publisher.PublishFailed(ctx, taskID, newSlotReconcileEvent(TaskModel.EventFailed, taskID, clientID, "", "fetch_slots_failed", TaskModel.StatusFailed, "slot reconcile failed", err.Error(), "check client /api/v1/edge/slots availability"))
		return
	}
	_ = publisher.PublishProgress(ctx, taskID, newSlotReconcileEvent(TaskModel.EventProgress, taskID, clientID, "", "fetch_slots", TaskModel.StatusRunning, "client slots loaded", "", ""))

	now := time.Now().Unix()
	rows := make([]NodeDAO.SlotRow, 0, len(slots))
	availableCount := int64(0)
	runningCount := int64(0)
	for _, slot := range slots {
		normalizedStatus := normalizeClientSlotStatus(slot.Status)
		if normalizedStatus == "waiting" {
			availableCount++
		}
		if normalizedStatus == "running" {
			runningCount++
		}
		rows = append(rows, NodeDAO.SlotRow{
			ClientID:      clientID,
			SlotID:        strings.TrimSpace(slot.SlotID),
			Status:        normalizedStatus,
			CurrentEnvID:  strings.TrimSpace(slot.CurrentPackageID),
			CurrentRunID:  strings.TrimSpace(slot.CurrentRunID),
			ContainerID:   strings.TrimSpace(slot.ContainerID),
			ContainerName: strings.TrimSpace(slot.ContainerName),
			CDPPort:       slot.CDPPort,
			VNCPort:       slot.VNCPort,
			LastError:     strings.TrimSpace(slot.LastError),
			LastSyncedAt:  now,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	repo := NodeRepo.NewRepository()
	if err = repo.ReplaceSlots(ctx, clientID, rows); err != nil {
		_ = publisher.PublishFailed(ctx, taskID, newSlotReconcileEvent(TaskModel.EventFailed, taskID, clientID, "", "replace_slots_failed", TaskModel.StatusFailed, "slot reconcile failed", err.Error(), "check node sqlite write path"))
		return
	}
	_ = publisher.PublishProgress(ctx, taskID, newSlotReconcileEvent(TaskModel.EventProgress, taskID, clientID, "", "replace_slots", TaskModel.StatusRunning, "node slot cache refreshed", "", ""))

	slotExceptionStatus := "normal"
	slotExceptionReason := ""
	actualSlotCount := int64(len(rows))
	if targetSlotCount > 0 && targetSlotCount != actualSlotCount {
		slotExceptionStatus = "exception"
		slotExceptionReason = fmt.Sprintf("target_slot_count=%d actual_slot_count=%d", targetSlotCount, actualSlotCount)
	}
	if err = repo.UpdateSlotSummary(ctx, &NodeDAO.Row{
		ClientID:            clientID,
		TargetSlotCount:     targetSlotCount,
		ActualSlotCount:     actualSlotCount,
		AvailableSlotCount:  availableCount,
		RunningSlotCount:    runningCount,
		SlotExceptionStatus: slotExceptionStatus,
		SlotExceptionReason: slotExceptionReason,
		LastSlotCheckedAt:   now,
		UpdatedAt:           now,
	}); err != nil {
		_ = publisher.PublishFailed(ctx, taskID, newSlotReconcileEvent(TaskModel.EventFailed, taskID, clientID, "", "update_summary_failed", TaskModel.StatusFailed, "slot reconcile failed", err.Error(), "check edge_clients summary fields"))
		return
	}

	_ = repo.CreateSlotLog(ctx, &NodeDAO.SlotLogRow{
		ClientID:  clientID,
		SlotID:    "",
		Action:    "slot_reconcile",
		Result:    "success",
		Message:   firstNonEmpty(source, "slot reconcile success"),
		CreatedAt: now,
	})

	_ = publisher.PublishCompleted(ctx, taskID, newSlotReconcileEvent(TaskModel.EventCompleted, taskID, clientID, "", "finalize_success", TaskModel.StatusSuccess, "slot reconcile completed", "", ""))
}

func newSlotReconcileEvent(eventType, taskID, clientID, slotID, stage, status, message, errMsg, suggestion string) TaskModel.Event {
	return TaskModel.Event{
		Event:        eventType,
		TaskID:       taskID,
		TaskType:     "slot_reconcile",
		ResourceType: "edge_client",
		ResourceID:   clientID,
		ClientID:     clientID,
		SlotID:       slotID,
		Stage:        stage,
		Status:       status,
		Message:      message,
		Error:        errMsg,
		Suggestion:   suggestion,
		Timestamp:    time.Now().Format(time.RFC3339),
	}
}

func normalizeClientSlotStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "waiting":
		return "waiting"
	case "loading":
		return "loading"
	case "running", "occupied":
		return "running"
	case "ending", "releasing":
		return "ending"
	default:
		return "waiting"
	}
}
