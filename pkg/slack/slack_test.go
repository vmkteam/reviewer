package slack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReviewNotificationText(t *testing.T) {
	base := ReviewNotification{
		ProjectTitle: "group/app",
		Title:        "ABC-1 Add upload",
		Author:       "john",
		SourceBranch: "ABC-1",
		TargetBranch: "main",
		TrafficLight: "yellow",
		IssueStats:   IssueStats{Medium: 7, Low: 11},
		ReviewURL:    "https://reviewer.example.com/reviews/5/",
	}

	tests := []struct {
		name string
		edit func(*ReviewNotification)
		want string
	}{
		{
			name: "without merge request",
			edit: func(*ReviewNotification) {},
			want: ":large_yellow_circle: [group/app] *<https://reviewer.example.com/reviews/5/|ABC-1 Add upload>* by john (`ABC-1` → `main`) — 0 critical, 0 high, 7 medium, 11 low",
		},
		{
			name: "with merge request",
			edit: func(n *ReviewNotification) {
				n.MRURL, n.MRLabel = "https://git.example.com/group/app/-/merge_requests/42", "MR !42"
			},
			want: ":large_yellow_circle: [group/app] <https://git.example.com/group/app/-/merge_requests/42|MR !42> *<https://reviewer.example.com/reviews/5/|ABC-1 Add upload>* by john (`ABC-1` → `main`) — 0 critical, 0 high, 7 medium, 11 low",
		},
		{
			name: "mrkdwn control characters are escaped",
			edit: func(n *ReviewNotification) {
				n.Title, n.Author, n.SourceBranch = "Use <T> & friends", "a>b", "x<y"
			},
			want: ":large_yellow_circle: [group/app] *<https://reviewer.example.com/reviews/5/|Use &lt;T&gt; &amp; friends>* by a&gt;b (`x&lt;y` → `main`) — 0 critical, 0 high, 7 medium, 11 low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := base
			tt.edit(&n)
			assert.Equal(t, tt.want, n.text())
		})
	}
}
