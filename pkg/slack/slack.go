package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vmkteam/embedlog"
)

// Notifier sends notifications to Slack via Incoming Webhook.
type Notifier struct {
	httpClient *http.Client
	embedlog.Logger
}

// NewNotifier creates a new Notifier.
func NewNotifier(logger embedlog.Logger) *Notifier {
	return &Notifier{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		Logger:     logger,
	}
}

// IssueStats contains issue counts by severity.
type IssueStats struct {
	Critical int
	High     int
	Medium   int
	Low      int
}

// ReviewNotification contains data for a Slack notification about a new review.
type ReviewNotification struct {
	WebhookURL   string
	ProjectTitle string
	ReviewID     int
	Title        string
	Author       string
	SourceBranch string
	TargetBranch string
	TrafficLight string
	IssueStats   IssueStats
	ReviewURL    string
}

func trafficLightEmoji(tl string) string {
	switch tl {
	case "red":
		return ":red_circle:"
	case "yellow":
		return ":large_yellow_circle:"
	case "green":
		return ":large_green_circle:"
	default:
		return ":white_circle:"
	}
}

func (n ReviewNotification) text() string {
	return fmt.Sprintf("%s [%s] *<%s|%s>* by %s (`%s` → `%s`) — %d critical, %d high, %d medium, %d low",
		trafficLightEmoji(n.TrafficLight),
		n.ProjectTitle,
		n.ReviewURL,
		n.Title,
		n.Author,
		n.SourceBranch,
		n.TargetBranch,
		n.IssueStats.Critical,
		n.IssueStats.High,
		n.IssueStats.Medium,
		n.IssueStats.Low,
	)
}

type slackMessage struct {
	Text string `json:"text"`
}

// Send sends a review notification to Slack. Logs errors without returning them.
func (n *Notifier) Send(ctx context.Context, notif ReviewNotification) {
	body, err := json.Marshal(slackMessage{Text: notif.text()})
	if err != nil {
		n.Error(ctx, "slack: marshal error", "err", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.WebhookURL, bytes.NewReader(body))
	if err != nil {
		n.Error(ctx, "slack: request error", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		n.Error(ctx, "slack: send error", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		n.Error(ctx, "slack: unexpected status", "status", resp.StatusCode, "body", string(respBody))
	}
}
