package Task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"private_browser_server/Pkg/HttpResponse"
	TaskRepo "private_browser_server/Repository/Task"
)

func GetDetail(c *gin.Context) {
	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := GetService().GetDetail(requestCtx, c.Param("taskId"))
	if err != nil {
		if err == TaskRepo.ErrNotFound {
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server task not found")
			return
		}
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

func SubscribeEvents(c *gin.Context) {
	snapshot, stream, cancel, err := GetService().Subscribe(c.Param("taskId"))
	if err != nil {
		if err == TaskRepo.ErrNotFound {
			c.JSON(http.StatusOK, gin.H{
				"code":    1004,
				"message": "server task not found",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    1005,
			"message": err.Error(),
		})
		return
	}
	defer cancel()

	writer := c.Writer
	header := writer.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	for _, event := range snapshot.Events {
		if err = writeSSEEvent(writer, event); err != nil {
			return
		}
	}
	if snapshot.Done || stream == nil {
		return
	}

	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			if err = writeSSEEvent(writer, event); err != nil {
				return
			}
		}
	}
}

func writeSSEEvent(writer gin.ResponseWriter, event any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal task event failed: %w", err)
	}
	if _, err = fmt.Fprintf(writer, "event: %s\n", extractEventName(event)); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(writer, "data: %s\n\n", body); err != nil {
		return err
	}
	writer.Flush()
	return nil
}

func extractEventName(event any) string {
	type eventNamer interface {
		GetEvent() string
	}
	if named, ok := event.(eventNamer); ok {
		return named.GetEvent()
	}
	return "message"
}
